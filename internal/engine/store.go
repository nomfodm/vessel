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
	if len(sha) < 2 {
		// An invalid SHA can't match any real object; return a path that never
		// exists so Has() returns false and Add() hits validateSha() cleanly.
		return filepath.Join(s.root, "_bad_", sha)
	}
	return filepath.Join(s.root, sha[:2], sha)
}

func (s *store) partPath(sha string) string {
	return s.objectPath(sha) + ".part"
}

func (s *store) Path(sha string) string {
	return s.objectPath(sha)
}

func (s *store) Has(sha string) bool {
	_, err := os.Stat(s.objectPath(sha))
	return err == nil
}

// PartSize returns the byte count of an in-progress download for sha, or 0 if
// none exists. The caller uses this as the Range offset for the next request.
func (s *store) PartSize(sha string) int64 {
	fi, err := os.Stat(s.partPath(sha))
	if err != nil {
		return 0
	}
	return fi.Size()
}

// Add streams r into the store starting at offset, hashing the whole object
// (existing .part bytes first, then new bytes from r), and only commits if the
// final hash matches wantSha.
//
// offset must equal the actual byte position the server started streaming from
// (0 for a fresh download, N for a resumed one where a .part of that size
// already exists). A hash mismatch deletes the .part so the next attempt starts
// clean; a network error leaves it intact for the next resume.
// countReader wraps a reader and calls fn with every chunk of bytes read.
type countReader struct {
	r  io.Reader
	fn func(int64)
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 && c.fn != nil {
		c.fn(int64(n))
	}
	return n, err
}

func (s *store) Add(r io.Reader, wantSha string, offset int64, onBytes func(int64)) error {
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

	part := s.partPath(wantSha)
	h := sha256.New()

	var f *os.File
	var err error
	if offset > 0 {
		// Catch the hasher up to the already-downloaded portion so the final
		// SHA256 covers the complete file, not just the tail.
		existing, oerr := os.Open(part)
		if oerr != nil {
			return fmt.Errorf("open part for rehash: %w", oerr)
		}
		if _, oerr = io.Copy(h, existing); oerr != nil {
			existing.Close()
			return fmt.Errorf("rehash existing part: %w", oerr)
		}
		existing.Close()
		f, err = os.OpenFile(part, os.O_WRONLY|os.O_APPEND, 0o644)
	} else {
		f, err = os.Create(part) // truncates any stale .part
	}
	if err != nil {
		return err
	}

	corrupt := false
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
		if corrupt {
			os.Remove(part)
		}
	}()

	if _, err := io.Copy(io.MultiWriter(f, h), &countReader{r: r, fn: onBytes}); err != nil {
		return err // network error — leave .part for the next resume attempt
	}
	closed = true
	if err := f.Close(); err != nil {
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != wantSha {
		corrupt = true
		return fmt.Errorf("hash mismatch: want %s, got %s", wantSha, got)
	}

	if err := os.Rename(part, s.objectPath(wantSha)); err != nil {
		return err
	}
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
