package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Fetcher interface {
	Fetch(ctx context.Context, sha string) (io.ReadCloser, error)
}

type ProgressFunc func(done, total int, file string)

func (e *Engine) Sync(ctx context.Context, m Manifest, enabledOptional []string, fetch Fetcher, onProgress ProgressFunc) error {
	desired := desiredFiles(m, enabledOptional)
	e.log.Info("sync start", "desired", len(desired), "strictDirs", m.StrictDirs)

	total := len(desired)
	for i, f := range desired {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := e.ensureFile(ctx, f, fetch); err != nil {
			return fmt.Errorf("file %s: %w", f.Path, err)
		}
		if onProgress != nil {
			onProgress(i+1, total, f.Path)
		}
	}

	if err := e.pruneStrictDirs(m, desired); err != nil {
		return err
	}
	e.log.Info("sync done", "files", total)
	return nil
}

func desiredFiles(m Manifest, enabledOptional []string) []ManifestFile {
	enabled := make(map[string]bool, len(enabledOptional))
	for _, id := range enabledOptional {
		enabled[id] = true
	}

	out := make([]ManifestFile, 0, len(m.Files))
	for _, f := range m.Files {
		if f.Optional && !enabled[f.ID] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// ensureFile guarantees the file exists at its profile path with the right hash:
// reuse if already correct, otherwise fetch into CAS (verified) and materialize.
func (e *Engine) ensureFile(ctx context.Context, f ManifestFile, fetch Fetcher) error {
	dst := filepath.Join(e.layout.Root, filepath.FromSlash(f.Path))

	if ok, _ := fileHasSHA(dst, f.SHA256); ok {
		return nil
	}

	if !e.store.Has(f.SHA256) {
		rc, err := fetch.Fetch(ctx, f.SHA256)
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := e.store.Add(rc, f.SHA256); err != nil {
			return err
		}
		e.log.Debug("fetched object", "sha", f.SHA256, "path", f.Path)
	}

	return e.mat.Materialize(e.store.Path(f.SHA256), dst)
}

// pruneStrictDirs removes foreign files inside strict dirs — anything on disk
// there that the desired set does not mention. Non-strict dirs are never touched.
func (e *Engine) pruneStrictDirs(m Manifest, desired []ManifestFile) error {
	keep := make(map[string]bool, len(desired))
	for _, f := range desired {
		keep[filepath.Clean(filepath.FromSlash(f.Path))] = true
	}

	for _, sd := range m.StrictDirs {
		dirAbs := filepath.Join(e.layout.Root, filepath.FromSlash(sd))
		err := filepath.WalkDir(dirAbs, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, rerr := filepath.Rel(e.layout.Root, path)
			if rerr != nil {
				return rerr
			}
			if keep[filepath.Clean(rel)] {
				return nil
			}
			e.log.Warn("removing foreign file in strict dir", "path", rel)
			return os.Remove(path)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func fileHasSHA(path, wantSha string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), wantSha), nil
}
