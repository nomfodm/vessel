package system

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// relocate moves the game data from old to new. It only touches dataDirs — never
// the old root itself, the config file or the (open) log file that may share the
// root on Windows. On the same volume it renames each data dir (instant, preserves
// the CAS hardlinks). Across volumes it copies only keepCrossVolume — the verified
// object store — and drops the rest, which re-materializes from it on next launch;
// this avoids both re-downloading and inflating hardlinks into full copies.
func relocate(old, new string, dataDirs, keepCrossVolume []string, log *slog.Logger) error {
	if err := os.MkdirAll(new, 0o755); err != nil {
		return err
	}
	same, err := sameVolume(old, new)
	if err != nil {
		return err
	}
	if same {
		log.Info("relocating by rename (same volume)", "from", old, "to", new)
		if err := moveDirs(old, new, dataDirs); err != nil {
			return err
		}
	} else {
		log.Info("relocating by copy (cross volume)", "from", old, "to", new, "keep", keepCrossVolume)
		if err := copyKeepDropDirs(old, new, dataDirs, keepCrossVolume); err != nil {
			return err
		}
	}
	// Best-effort: drop the old root if our move emptied it. When it still holds
	// config/logs (Windows default layout) Remove fails on a non-empty dir and we
	// ignore that.
	_ = os.Remove(old)
	return nil
}

// moveDirs renames each existing data dir from old into new. Missing dirs are
// skipped.
func moveDirs(old, new string, dirs []string) error {
	for _, d := range dirs {
		src := filepath.Join(old, d)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := os.Rename(src, filepath.Join(new, d)); err != nil {
			return err
		}
	}
	return nil
}

// copyKeepDropDirs copies the keep dirs (the object store) cross-volume, then
// removes every data dir from old. Non-kept dirs (profiles/runtimes) are simply
// dropped — they rebuild from the objects on next launch.
func copyKeepDropDirs(old, new string, dataDirs, keep []string) error {
	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	for _, d := range dataDirs {
		src := filepath.Join(old, d)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if keepSet[d] {
			if err := copyTree(src, filepath.Join(new, d)); err != nil {
				return err
			}
		}
		if err := os.RemoveAll(src); err != nil {
			return err
		}
	}
	return nil
}

// sameVolume reports whether old and new live on the same filesystem, by probing
// a real rename — the operation we actually rely on. A failed probe rename (e.g.
// cross-device) just means "use the copy path", not a hard error.
func sameVolume(old, new string) (bool, error) {
	probe := filepath.Join(old, ".vessel-move-probe")
	if err := os.WriteFile(probe, []byte{}, 0o644); err != nil {
		return false, err
	}
	defer os.Remove(probe)

	dst := filepath.Join(new, ".vessel-move-probe")
	if err := os.Rename(probe, dst); err != nil {
		return false, nil
	}
	os.Remove(dst)
	return true, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}

	// Write to a temp file in the same directory as dst so the final rename is
	// within the same volume and therefore atomic. A crash mid-copy leaves a
	// stale .tmp that is cleaned up on the next attempt; it never masquerades as
	// a valid CAS object (which would cause store.Has to lie).
	tmp, err := os.CreateTemp(filepath.Dir(dst), "tmp-copy-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		tmp.Close()
		if !committed {
			os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Preserve source permissions so CAS objects keep their execute bits on
	// Linux/macOS (copyMaterializer propagates the object's mode to the target).
	if err := os.Chmod(tmpName, srcInfo.Mode()); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	committed = true
	return nil
}
