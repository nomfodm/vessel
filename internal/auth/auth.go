package auth

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// refreshSkew refreshes the access token a bit before it actually expires, so a
// session handed to the game is not on the edge of expiry.
const refreshSkew = 30 * time.Second

var ErrNotLoggedIn = errors.New("not logged in")

type User struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatarUrl"`
}

// GameSession is what the launcher passes to the game's auth arguments. Its
// AccessToken is the game token (a short opaque string), NOT the launcher JWT.
type GameSession struct {
	UUID        string `json:"uuid"`
	Username    string `json:"username"`
	AccessToken string `json:"accessToken"`
}

// Tokens are the launcher-side credentials: Access is a JWT for the launcher
// API, Refresh renews it. Neither is the game token.
type Tokens struct {
	Access    string
	Refresh   string
	ExpiresAt time.Time
}

type Backend interface {
	Login(ctx context.Context, username, password string) (Tokens, User, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, User, error)
	// IssueGameSession exchanges a valid launcher JWT for a fresh game session.
	IssueGameSession(ctx context.Context, launcherAccessToken string) (GameSession, error)
}

// TokenStore persists only the long-lived refresh token. Load returns "" when
// nothing is stored.
type TokenStore interface {
	Save(refresh string) error
	Load() (string, error)
	Delete() error
}

type Service struct {
	mu      sync.Mutex
	backend Backend
	store   TokenStore
	log     *slog.Logger

	user   *User
	tokens Tokens
}

func New(backend Backend, store TokenStore, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		backend: backend,
		store:   store,
		log:     log.With("svc", "auth"),
	}
}

func (s *Service) Login(ctx context.Context, username, password string) error {
	tokens, user, err := s.backend.Login(ctx, username, password)
	if err != nil {
		s.log.Warn("login failed", "username", username, "err", err)
		return err
	}

	s.mu.Lock()
	s.tokens = tokens
	s.user = &user
	s.mu.Unlock()

	if err := s.store.Save(tokens.Refresh); err != nil {
		s.log.Error("persist refresh token", "err", err)
	}
	s.log.Info("login ok", "username", user.Username, "uuid", user.UUID)
	return nil
}

// Restore re-establishes a session from a stored refresh token (startup path).
// Returns false (no error) when there is nothing stored to restore.
func (s *Service) Restore(ctx context.Context) (bool, error) {
	refresh, err := s.store.Load()
	if err != nil {
		s.log.Error("load refresh token", "err", err)
		return false, err
	}
	if refresh == "" {
		return false, nil
	}

	tokens, user, err := s.backend.Refresh(ctx, refresh)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			// Refresh token is genuinely invalid — clear it so the user is prompted
			// to log in again rather than retrying forever.
			s.log.Warn("restore failed: token rejected, clearing", "err", err)
			_ = s.store.Delete()
		} else {
			// Transient error (network down, server restarting) — keep the token
			// so the next launch attempt can try again without forcing re-login.
			s.log.Warn("restore failed: network error, keeping token", "err", err)
		}
		return false, nil
	}

	s.mu.Lock()
	s.tokens = tokens
	s.user = &user
	s.mu.Unlock()
	s.persistRefreshed(tokens.Refresh, refresh)

	s.log.Info("session restored", "username", user.Username)
	return true, nil
}

func (s *Service) Logout() error {
	s.mu.Lock()
	s.user = nil
	s.tokens = Tokens{}
	s.mu.Unlock()
	s.log.Info("logout")
	return s.store.Delete()
}

func (s *Service) IsLoggedIn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user != nil
}

func (s *Service) CurrentUser() (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil {
		return User{}, ErrNotLoggedIn
	}
	return *s.user, nil
}

// GameSession issues a fresh game session for launching: it first ensures the
// launcher JWT is valid (refreshing if stale), then exchanges it for a new game
// token. The game token is never cached — a new one each launch.
func (s *Service) GameSession(ctx context.Context) (GameSession, error) {
	jwt, err := s.validLauncherToken(ctx)
	if err != nil {
		return GameSession{}, err
	}
	gs, err := s.backend.IssueGameSession(ctx, jwt)
	if err != nil {
		s.log.Error("issue game session failed", "err", err)
		return GameSession{}, err
	}
	s.log.Info("game session issued", "username", gs.Username)
	return gs, nil
}

// validLauncherToken returns a non-expired launcher JWT, refreshing it if it is
// expired or about to expire.
func (s *Service) validLauncherToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.user == nil {
		s.mu.Unlock()
		return "", ErrNotLoggedIn
	}
	if time.Now().Before(s.tokens.ExpiresAt.Add(-refreshSkew)) {
		jwt := s.tokens.Access
		s.mu.Unlock()
		return jwt, nil
	}
	oldRefresh := s.tokens.Refresh
	s.mu.Unlock()

	s.log.Info("launcher token stale, refreshing")
	tokens, user, err := s.backend.Refresh(ctx, oldRefresh)
	if err != nil {
		s.log.Error("refresh failed", "err", err)
		return "", err
	}

	s.mu.Lock()
	s.tokens = tokens
	s.user = &user
	s.mu.Unlock()
	s.persistRefreshed(tokens.Refresh, oldRefresh)

	return tokens.Access, nil
}

// persistRefreshed re-saves the refresh token only if the backend rotated it.
func (s *Service) persistRefreshed(newRefresh, old string) {
	if newRefresh == "" || newRefresh == old {
		return
	}
	if err := s.store.Save(newRefresh); err != nil {
		s.log.Error("persist rotated refresh token", "err", err)
	}
}
