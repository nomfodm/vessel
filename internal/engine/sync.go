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
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

type Fetcher interface {
	// Fetch opens the object identified by sha for reading starting at offset
	// bytes. The caller supplies the size of any existing .part file as offset;
	// if the server honours the Range request it returns 206 and the actual
	// offset the stream begins at; if it ignores Range it returns 200 and
	// actual=0 (the caller must discard the .part and start fresh).
	Fetch(ctx context.Context, sha string, offset int64) (body io.ReadCloser, actual int64, err error)
}

// ProgressFunc reports sync progress in bytes (done/total of the desired set),
// plus the path just processed. Byte-based so the UI can show real download size.
type ProgressFunc func(doneBytes, totalBytes int64, file string)

func (e *Engine) Sync(ctx context.Context, m Manifest, stateKey string, enabledOptional []string, fetch Fetcher, onProgress ProgressFunc) error {
	desired := desiredFiles(m, enabledOptional)
	e.log.Info("sync start", "desired", len(desired), "strictDirs", m.StrictDirs, "key", stateKey)

	var totalBytes int64
	for _, f := range desired {
		totalBytes += f.Size
	}

	statePath := filepath.Join(e.layout.Root, ".sync-states", stateKey+".json")
	old := loadState(statePath)

	const workers = 8
	var doneBytes atomic.Int64
	var stateMu sync.Mutex
	newFiles := make(map[string]fileState, len(desired))

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, workers)

	for _, f := range desired {
		f := f
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-sem }()

			onBytes := func(n int64) {
				total := doneBytes.Add(n)
				if onProgress != nil {
					onProgress(total, totalBytes, f.Path)
				}
			}
			if err := e.ensureFile(gctx, f, fetch, onBytes, old, &stateMu, newFiles); err != nil {
				return fmt.Errorf("file %s: %w", f.Path, err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := e.pruneStrictDirs(m, desired); err != nil {
		return err
	}

	_ = saveState(statePath, SyncState{ManifestSHA: manifestFingerprint(desired), Files: newFiles})
	e.log.Info("sync done", "files", len(desired), "bytes", totalBytes)
	return nil
}

// DesiredBytes is the total size of the files Sync would materialize for the
// given optional selection. Lets a caller size a unified progress bar that spans
// several manifests before any sync starts.
func DesiredBytes(m Manifest, enabledOptional []string) int64 {
	var n int64
	for _, f := range desiredFiles(m, enabledOptional) {
		n += f.Size
	}
	return n
}

func desiredFiles(m Manifest, enabledOptional []string) []ManifestFile {
	// nil means "never configured" — fall back to each group's DefaultOn.
	// An empty non-nil slice means the user explicitly chose nothing optional.
	if enabledOptional == nil {
		for _, g := range m.Optional {
			if g.DefaultOn {
				enabledOptional = append(enabledOptional, g.ID)
			}
		}
	}

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

// ensureFile guarantees the file at dst has the right hash, using a layered strategy:
//
//  1. Stat cache: if size, mtime and expected SHA all agree with the stored state,
//     skip hashing entirely — fastest path, zero disk reads.
//  2. Hash check: cache miss or stale mtime — hash to confirm before downloading.
//  3. Download: fetch into CAS (with resume), then materialise to dst.
func (e *Engine) ensureFile(ctx context.Context, f ManifestFile, fetch Fetcher, onBytes func(int64), old SyncState, mu *sync.Mutex, newFiles map[string]fileState) error {
	dst := filepath.Join(e.layout.Root, filepath.FromSlash(f.Path))

	// Stat cache hit: size, mtime, and expected SHA all agree — no disk read needed.
	if cached, ok := old.Files[f.Path]; ok && cached.SHA == f.SHA256 {
		if info, err := os.Stat(dst); err == nil && info.Size() == f.Size && info.ModTime().Equal(cached.Mtime) {
			mu.Lock()
			newFiles[f.Path] = cached
			mu.Unlock()
			onBytes(f.Size)
			return nil
		}
	}

	// File exists: hash it to confirm correctness before downloading.
	if ok, _ := fileHasSHA(dst, f.SHA256); ok {
		if info, err := os.Stat(dst); err == nil {
			mu.Lock()
			newFiles[f.Path] = fileState{Size: f.Size, Mtime: info.ModTime(), SHA: f.SHA256}
			mu.Unlock()
		}
		onBytes(f.Size)
		return nil
	}

	// Need to download — fetch into CAS, then materialise.
	if !e.store.Has(f.SHA256) {
		_, err, shared := e.sf.Do(f.SHA256, func() (any, error) {
			offset := e.store.PartSize(f.SHA256)
			rc, actual, err := fetch.Fetch(ctx, f.SHA256, offset)
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			if err := e.store.Add(rc, f.SHA256, actual, onBytes); err != nil {
				return nil, err
			}
			e.log.Debug("fetched object", "sha", f.SHA256, "path", f.Path, "resumedFrom", actual)
			return nil, nil
		})
		if err != nil {
			return err
		}
		if shared {
			onBytes(f.Size)
		}
	} else {
		onBytes(f.Size)
	}

	if err := e.mat.Materialize(e.store.Path(f.SHA256), dst); err != nil {
		return err
	}

	// Record fresh stat so the next launch can skip hashing this file.
	if info, err := os.Stat(dst); err == nil {
		mu.Lock()
		newFiles[f.Path] = fileState{Size: f.Size, Mtime: info.ModTime(), SHA: f.SHA256}
		mu.Unlock()
	}
	return nil
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
