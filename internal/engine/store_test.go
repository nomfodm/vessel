package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestStoreAddHasPath(t *testing.T) {
	s := newStore(t.TempDir())
	content := []byte("hello vessel")
	sha := sha256hex(content)

	if s.Has(sha) {
		t.Fatal("Has true before Add")
	}
	if err := s.Add(strings.NewReader(string(content)), sha); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !s.Has(sha) {
		t.Fatal("Has false after Add")
	}

	if want := filepath.Join(s.root, sha[:2], sha); s.Path(sha) != want {
		t.Errorf("Path = %q, want sharded %q", s.Path(sha), want)
	}
	got, err := os.ReadFile(s.Path(sha))
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("object content = %q, want %q", got, content)
	}
}

func TestStoreAddHashMismatch(t *testing.T) {
	s := newStore(t.TempDir())
	wrong := sha256hex([]byte("something else"))

	err := s.Add(strings.NewReader("actual content"), wrong)
	if err == nil {
		t.Fatal("expected hash mismatch error, got nil")
	}
	if s.Has(wrong) {
		t.Error("corrupt object was committed to the store")
	}
}

func TestStoreAddDedup(t *testing.T) {
	s := newStore(t.TempDir())
	content := []byte("dup me")
	sha := sha256hex(content)

	if err := s.Add(strings.NewReader(string(content)), sha); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := s.Add(strings.NewReader(string(content)), sha); err != nil {
		t.Fatalf("second Add (dedup): %v", err)
	}

	entries, _ := os.ReadDir(filepath.Dir(s.Path(sha)))
	if len(entries) != 1 {
		t.Errorf("got %d files in shard, want 1 (no temp leftovers)", len(entries))
	}
}

func TestValidateSha(t *testing.T) {
	good := sha256hex([]byte("x"))
	if err := validateSha(good); err != nil {
		t.Errorf("valid sha rejected: %v", err)
	}
	for _, bad := range []string{"", "abc", strings.Repeat("g", 64), strings.ToUpper(good)} {
		if err := validateSha(bad); err == nil {
			t.Errorf("invalid sha %q accepted", bad)
		}
	}
}
