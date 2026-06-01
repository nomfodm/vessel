package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type store struct {
	root string
}

func newStore(root string) *store {
	return &store{root: root}
}

func (s *store) objectPath(sha string) string {
	return filepath.Join(s.root, sha[:2], sha)
}

func (s *store) Path(sha string) string {
	return s.objectPath(sha)
}

func (s *store) Has(sha string) bool {
	_, err := os.Stat(s.objectPath(sha))
	return err == nil
}

// Add streams r into the store while hashing it, and only commits if the hash
// matches wantSha — a corrupt download never lands as a valid object. wantSha
// must be lowercase hex sha256. Already-present objects are a no-op (dedup).
func (s *store) Add(r io.Reader, wantSha string) error {
	if err := validateSha(wantSha); err != nil {
		return err
	}
	if s.Has(wantSha) {
		return nil
	}

	dir := filepath.Dir(s.objectPath(wantSha))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		tmp.Close()
		if !renamed {
			os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), r); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != wantSha {
		return fmt.Errorf("hash mismatch: want %s, got %s", wantSha, got)
	}

	if err := os.Rename(tmpName, s.objectPath(wantSha)); err != nil {
		return err
	}
	renamed = true
	return nil
}

func validateSha(sha string) error {
	if len(sha) != 64 {
		return fmt.Errorf("invalid sha length %d (want 64)", len(sha))
	}
	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("invalid sha char %q", c)
		}
	}
	return nil
}
