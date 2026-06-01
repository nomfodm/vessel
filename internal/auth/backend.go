package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// httpBackend talks to the infinityserver.ru auth API. Endpoint paths and the
// response shape are isolated here — adjust to the real API without touching
// Service.
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

type authResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresInSec int    `json:"expiresIn"`
	UUID         string `json:"uuid"`
	Username     string `json:"username"`
}

func (b *httpBackend) Login(ctx context.Context, username, password string) (Tokens, User, error) {
	return b.post(ctx, "/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
}

func (b *httpBackend) Refresh(ctx context.Context, refreshToken string) (Tokens, User, error) {
	return b.post(ctx, "/auth/refresh", map[string]string{
		"refreshToken": refreshToken,
	})
}

type gameSessionResponse struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	GameToken string `json:"gameToken"`
}

func (b *httpBackend) IssueGameSession(ctx context.Context, launcherAccessToken string) (GameSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/auth/game-session", nil)
	if err != nil {
		return GameSession{}, err
	}
	req.Header.Set("Authorization", "Bearer "+launcherAccessToken)

	resp, err := b.http.Do(req)
	if err != nil {
		return GameSession{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GameSession{}, fmt.Errorf("issue game session: status %d", resp.StatusCode)
	}

	var gr gameSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return GameSession{}, err
	}
	return GameSession{UUID: gr.UUID, Username: gr.Username, AccessToken: gr.GameToken}, nil
}

func (b *httpBackend) post(ctx context.Context, path string, body any) (Tokens, User, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return Tokens{}, User{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return Tokens{}, User{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.http.Do(req)
	if err != nil {
		return Tokens{}, User{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, User{}, fmt.Errorf("auth %s: status %d", path, resp.StatusCode)
	}

	var ar authResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return Tokens{}, User{}, err
	}
	tokens := Tokens{
		Access:    ar.AccessToken,
		Refresh:   ar.RefreshToken,
		ExpiresAt: time.Now().Add(time.Duration(ar.ExpiresInSec) * time.Second),
	}
	return tokens, User{UUID: ar.UUID, Username: ar.Username}, nil
}
