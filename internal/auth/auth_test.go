package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeBackend is a scriptable Backend.
type fakeBackend struct {
	loginTokens  Tokens
	loginUser    User
	loginErr     error
	refreshFn    func(refresh string) (Tokens, User, error)
	refreshCalls int
	issueFn      func(jwt string) (GameSession, error)
	issuedWith   string
}

func (f *fakeBackend) Login(_ context.Context, _, _ string) (Tokens, User, error) {
	return f.loginTokens, f.loginUser, f.loginErr
}

func (f *fakeBackend) Refresh(_ context.Context, refresh string) (Tokens, User, error) {
	f.refreshCalls++
	if f.refreshFn != nil {
		return f.refreshFn(refresh)
	}
	return Tokens{}, User{}, errors.New("no refresh scripted")
}

func (f *fakeBackend) IssueGameSession(_ context.Context, jwt string) (GameSession, error) {
	f.issuedWith = jwt
	if f.issueFn != nil {
		return f.issueFn(jwt)
	}
	return GameSession{}, errors.New("no issue scripted")
}

// issueEcho is a default IssueGameSession that returns a game token derived from
// the JWT it was given, so tests can assert which launcher token was used.
func issueEcho(jwt string) (GameSession, error) {
	return GameSession{UUID: "u-1", Username: "alex", AccessToken: "game-for:" + jwt}, nil
}

// memStore is an in-memory TokenStore.
type memStore struct {
	val    string
	loaded bool
}

func (m *memStore) Save(r string) error   { m.val = r; m.loaded = true; return nil }
func (m *memStore) Load() (string, error) { return m.val, nil }
func (m *memStore) Delete() error         { m.val = ""; m.loaded = false; return nil }

func TestLoginPersistsAndSetsUser(t *testing.T) {
	be := &fakeBackend{
		loginTokens: Tokens{Access: "acc", Refresh: "ref", ExpiresAt: time.Now().Add(time.Hour)},
		loginUser:   User{UUID: "u-1", Username: "alex"},
	}
	store := &memStore{}
	s := New(be, store, testLogger())

	if err := s.Login(context.Background(), "alex", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !s.IsLoggedIn() {
		t.Error("not logged in after Login")
	}
	if store.val != "ref" {
		t.Errorf("refresh token not persisted, got %q", store.val)
	}
	u, _ := s.CurrentUser()
	if u.UUID != "u-1" {
		t.Errorf("user = %+v", u)
	}
}

func TestGameSessionFreshNoRefresh(t *testing.T) {
	be := &fakeBackend{
		loginTokens: Tokens{Access: "jwt-acc", Refresh: "ref", ExpiresAt: time.Now().Add(time.Hour)},
		loginUser:   User{UUID: "u-1", Username: "alex"},
		issueFn:     issueEcho,
	}
	s := New(be, &memStore{}, testLogger())
	_ = s.Login(context.Background(), "alex", "pw")

	gs, err := s.GameSession(context.Background())
	if err != nil {
		t.Fatalf("GameSession: %v", err)
	}
	// Game token is issued from the (fresh) launcher JWT, not equal to it.
	if gs.AccessToken != "game-for:jwt-acc" {
		t.Errorf("game token = %q, want issued from jwt-acc", gs.AccessToken)
	}
	if be.issuedWith != "jwt-acc" {
		t.Errorf("issued with %q, want the launcher JWT jwt-acc", be.issuedWith)
	}
	if be.refreshCalls != 0 {
		t.Errorf("refreshed a fresh token (%d calls)", be.refreshCalls)
	}
}

func TestGameSessionRefreshesStaleThenIssues(t *testing.T) {
	be := &fakeBackend{
		loginTokens: Tokens{Access: "old-jwt", Refresh: "ref1", ExpiresAt: time.Now().Add(-time.Minute)},
		loginUser:   User{UUID: "u-1", Username: "alex"},
		refreshFn: func(refresh string) (Tokens, User, error) {
			if refresh != "ref1" {
				return Tokens{}, User{}, errors.New("wrong refresh token")
			}
			return Tokens{Access: "new-jwt", Refresh: "ref2", ExpiresAt: time.Now().Add(time.Hour)},
				User{UUID: "u-1", Username: "alex"}, nil
		},
		issueFn: issueEcho,
	}
	store := &memStore{}
	s := New(be, store, testLogger())
	_ = s.Login(context.Background(), "alex", "pw")

	gs, err := s.GameSession(context.Background())
	if err != nil {
		t.Fatalf("GameSession: %v", err)
	}
	// Stale launcher JWT was refreshed first, then the game token issued from new-jwt.
	if be.issuedWith != "new-jwt" {
		t.Errorf("issued with %q, want refreshed new-jwt", be.issuedWith)
	}
	if gs.AccessToken != "game-for:new-jwt" {
		t.Errorf("game token = %q", gs.AccessToken)
	}
	if store.val != "ref2" {
		t.Errorf("rotated refresh not persisted, got %q", store.val)
	}
}

func TestRestoreFromStoredToken(t *testing.T) {
	be := &fakeBackend{
		refreshFn: func(refresh string) (Tokens, User, error) {
			return Tokens{Access: "acc", Refresh: refresh, ExpiresAt: time.Now().Add(time.Hour)},
				User{UUID: "u-9", Username: "restored"}, nil
		},
	}
	store := &memStore{val: "stored-ref"}
	s := New(be, store, testLogger())

	ok, err := s.Restore(context.Background())
	if err != nil || !ok {
		t.Fatalf("Restore = %v, %v", ok, err)
	}
	if !s.IsLoggedIn() {
		t.Error("not logged in after Restore")
	}
}

func TestRestoreNothingStored(t *testing.T) {
	s := New(&fakeBackend{}, &memStore{}, testLogger())
	ok, err := s.Restore(context.Background())
	if err != nil || ok {
		t.Fatalf("Restore with empty store = %v, %v; want false, nil", ok, err)
	}
}

func TestRestoreInvalidTokenClears(t *testing.T) {
	be := &fakeBackend{
		refreshFn: func(string) (Tokens, User, error) {
			// Real backend wraps ErrUnauthorized when server returns 401/403.
			return Tokens{}, User{}, fmt.Errorf("POST /v1/launcher/auth/refresh: status 401: %w", ErrUnauthorized)
		},
	}
	store := &memStore{val: "bad-ref"}
	s := New(be, store, testLogger())

	ok, err := s.Restore(context.Background())
	if err != nil || ok {
		t.Fatalf("Restore = %v, %v; want false, nil", ok, err)
	}
	if store.val != "" {
		t.Error("invalid token not cleared from store")
	}
}

func TestRestoreNetworkErrorKeepsToken(t *testing.T) {
	be := &fakeBackend{
		refreshFn: func(string) (Tokens, User, error) {
			// Network failure — NOT an auth rejection.
			return Tokens{}, User{}, errors.New("connection refused")
		},
	}
	store := &memStore{val: "good-ref"}
	s := New(be, store, testLogger())

	ok, err := s.Restore(context.Background())
	if err != nil || ok {
		t.Fatalf("Restore = %v, %v; want false, nil", ok, err)
	}
	if store.val != "good-ref" {
		t.Error("token should be preserved on transient network error")
	}
}

func TestLogoutClears(t *testing.T) {
	be := &fakeBackend{
		loginTokens: Tokens{Access: "acc", Refresh: "ref", ExpiresAt: time.Now().Add(time.Hour)},
		loginUser:   User{UUID: "u-1", Username: "alex"},
	}
	store := &memStore{}
	s := New(be, store, testLogger())
	_ = s.Login(context.Background(), "alex", "pw")

	if err := s.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if s.IsLoggedIn() {
		t.Error("still logged in after Logout")
	}
	if store.val != "" {
		t.Error("refresh token not deleted on Logout")
	}
	if _, err := s.GameSession(context.Background()); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("GameSession after logout = %v, want ErrNotLoggedIn", err)
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.enc")
	fs := newFileStore(path, "machine-xyz")

	if err := fs.Save("secret-refresh"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "secret-refresh" {
		t.Errorf("Load = %q, want secret-refresh", got)
	}

	// Wrong key material must not decrypt.
	other := newFileStore(path, "different-machine")
	if _, err := other.Load(); err == nil {
		t.Error("token decrypted with wrong key material")
	}

	if err := fs.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if v, _ := fs.Load(); v != "" {
		t.Errorf("Load after delete = %q, want empty", v)
	}
}

func TestFileStoreEmptyOnMissing(t *testing.T) {
	fs := newFileStore(filepath.Join(t.TempDir(), "nope.enc"), "k")
	v, err := fs.Load()
	if err != nil || v != "" {
		t.Errorf("Load missing = %q, %v; want empty, nil", v, err)
	}
}
