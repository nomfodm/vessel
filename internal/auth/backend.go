package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpBackend talks to the infinityserver.ru launcher API (base + /v1/...).
// Endpoint paths and response shapes are isolated here — adjust to the real API
// without touching Service.
type httpBackend struct {
	baseURL string
	http    *http.Client
}

func NewBackend(baseURL string, client *http.Client) Backend {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &httpBackend{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

// tokenPair is the login/refresh response.
type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// userMeResponse is the subset of GET /v1/user/me we need.
type userMeResponse struct {
	Username         string `json:"username"`
	MinecraftProfile struct {
		UUID     string `json:"uuid"`
		Nickname string `json:"nickname"`
	} `json:"minecraft_profile"`
}

// sessionResponse is POST /v1/launcher/sessions (CreateMinecraftSession).
type sessionResponse struct {
	AccessToken string `json:"access_token"`
	ProfileUUID string `json:"profile_uuid"`
	Nickname    string `json:"nickname"`
}

func (b *httpBackend) Login(ctx context.Context, username, password string) (Tokens, User, error) {
	var tp tokenPair
	if err := b.do(ctx, http.MethodPost, "/v1/launcher/auth/login",
		map[string]string{"username": username, "password": password}, "", &tp); err != nil {
		return Tokens{}, User{}, err
	}
	user, err := b.me(ctx, tp.AccessToken)
	if err != nil {
		return Tokens{}, User{}, err
	}
	return tokensFrom(tp), user, nil
}

func (b *httpBackend) Refresh(ctx context.Context, refreshToken string) (Tokens, User, error) {
	var tp tokenPair
	if err := b.do(ctx, http.MethodPost, "/v1/launcher/auth/refresh",
		map[string]string{"refresh_token": refreshToken}, "", &tp); err != nil {
		return Tokens{}, User{}, err
	}
	user, err := b.me(ctx, tp.AccessToken)
	if err != nil {
		return Tokens{}, User{}, err
	}
	return tokensFrom(tp), user, nil
}

func (b *httpBackend) me(ctx context.Context, jwt string) (User, error) {
	var m userMeResponse
	if err := b.do(ctx, http.MethodGet, "/v1/user/me", nil, jwt, &m); err != nil {
		return User{}, err
	}
	return User{UUID: m.MinecraftProfile.UUID, Username: m.Username}, nil
}

func (b *httpBackend) IssueGameSession(ctx context.Context, launcherAccessToken string) (GameSession, error) {
	var s sessionResponse
	if err := b.do(ctx, http.MethodPost, "/v1/launcher/sessions", nil, launcherAccessToken, &s); err != nil {
		return GameSession{}, err
	}
	return GameSession{UUID: s.ProfileUUID, Username: s.Nickname, AccessToken: s.AccessToken}, nil
}

// do performs an HTTP call: marshals body (if any), sets Bearer (if jwt != ""),
// and decodes a 2xx JSON response into dst.
func (b *httpBackend) do(ctx context.Context, method, path string, body any, jwt string, dst any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}
	if dst == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func tokensFrom(tp tokenPair) Tokens {
	return Tokens{
		Access:    tp.AccessToken,
		Refresh:   tp.RefreshToken,
		ExpiresAt: jwtExp(tp.AccessToken),
	}
}

// jwtExp reads the "exp" claim from a JWT without verifying the signature (HTTPS
// protects transit; the server verifies). Falls back to a short lifetime when the
// token cannot be parsed, so a refresh happens soon rather than never.
func jwtExp(token string) time.Time {
	fallback := time.Now().Add(15 * time.Minute)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fallback
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fallback
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return fallback
	}
	return time.Unix(claims.Exp, 0)
}
