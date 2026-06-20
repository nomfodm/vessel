// Package baker builds the artifacts the launcher downloads: it hashes a local
// client directory into a content-addressed store (files/<ab>/<sha>) and writes
// the manifest the engine consumes. Output mirrors the S3 layout 1:1 so the
// result can be uploaded verbatim (mc/aws/rclone).
package baker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/nomfodm/vessel/internal/engine"
)

// OptionalRule marks files as optional by glob and tags them with an id that
// config.enabledOptional references. Name/Desc/DefaultOn are carried into the
// manifest for the UI toggle.
type OptionalRule struct {
	ID        string `json:"id"`
	Glob      string `json:"glob"`
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	DefaultOn bool   `json:"defaultOn"`
}

// Spec carries the manifest fields that cannot be derived from the files alone.
type Spec struct {
	Runtime          string         `json:"runtime"`
	RecommendedRamMB int            `json:"recommendedRamMB"`
	MinRamMB         int            `json:"minRamMB"`
	StrictDirs       []string       `json:"strictDirs"`
	Optional         []OptionalRule `json:"optional"`
}

type Result struct {
	Manifest     engine.Manifest
	TotalFiles   int
	NewObjects   int // objects written this run (dedup-skipped ones excluded)
	TotalBytes   int64
	ManifestPath string // written path, relative to outDir
}

// LoadSpec reads a baker.json spec.
func LoadSpec(specPath string) (Spec, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return Spec{}, err
	}
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	for _, r := range s.Optional {
		if r.ID == "" || r.Glob == "" {
			return Spec{}, fmt.Errorf("optional rule needs both id and glob: %+v", r)
		}
		if _, err := path.Match(r.Glob, ""); err != nil {
			return Spec{}, fmt.Errorf("optional %q: bad glob %q: %w", r.ID, r.Glob, err)
		}
	}
	return s, nil
}

// Bake hashes clientDir into outDir/files and writes the manifest to
// outDir/<manifestRel>. Every file path (and strict dir) is prefixed with
// pathPrefix so it materializes at the right place under the launcher's Root —
// "profiles/<slug>" for a profile, "runtimes/<name>" for a JRE. specSkip is the
// spec filename to ignore when it lives inside clientDir.
func Bake(log *slog.Logger, clientDir, pathPrefix, outDir, manifestRel, specSkip string, spec Spec) (Result, error) {
	if log == nil {
		log = slog.Default()
	}
	log = log.With("cmd", "baker")

	filesDir := filepath.Join(outDir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return Result{}, err
	}

	var files []engine.ManifestFile
	var newObjects int
	var totalBytes int64
	matched := make(map[string]bool) // optional ids that hit at least one file

	err := filepath.WalkDir(clientDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(clientDir, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == specSkip {
			return nil
		}

		// Single pass: hash while copying so the stored object is guaranteed to
		// match the SHA written into the manifest (no TOCTOU, no silent corruption).
		sha, size, wrote, err := hashAndStore(filesDir, p)
		if err != nil {
			return fmt.Errorf("store %s: %w", relSlash, err)
		}

		// Optional globs are authored relative to clientDir, so match on the
		// unprefixed path; store the prefixed one.
		optional, id := matchOptional(relSlash, spec.Optional)
		if optional {
			matched[id] = true
		}
		files = append(files, engine.ManifestFile{
			Path:     path.Join(pathPrefix, relSlash),
			SHA256:   sha,
			Size:     size,
			Optional: optional,
			ID:       id,
		})

		if wrote {
			newObjects++
			log.Debug("stored object", "sha", sha, "path", relSlash, "optional", optional)
		}
		totalBytes += size
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	// Emit a group only for rules that actually matched a file — otherwise a
	// phantom toggle for an absent component. Preserves spec order.
	var groups []engine.OptionalGroup
	for _, r := range spec.Optional {
		if !matched[r.ID] {
			log.Warn("optional rule matched no files, skipping", "id", r.ID, "glob", r.Glob)
			continue
		}
		name := r.Name
		if name == "" {
			name = r.ID
		}
		groups = append(groups, engine.OptionalGroup{ID: r.ID, Name: name, Desc: r.Desc, DefaultOn: r.DefaultOn})
	}

	// Strict dirs are authored relative to clientDir too; prefix them so prune
	// targets the right subtree on disk.
	strictDirs := make([]string, len(spec.StrictDirs))
	for i, sd := range spec.StrictDirs {
		strictDirs[i] = path.Join(pathPrefix, sd)
	}

	manifest := engine.Manifest{
		Files:            files,
		Optional:         groups,
		StrictDirs:       strictDirs,
		Runtime:          spec.Runtime,
		RecommendedRAMMB: spec.RecommendedRamMB,
		MinRAMMB:         spec.MinRamMB,
	}

	if err := writeManifest(filepath.Join(outDir, filepath.FromSlash(manifestRel)), manifest); err != nil {
		return Result{}, err
	}

	log.Info("bake done", "prefix", pathPrefix, "files", len(files), "newObjects", newObjects, "manifest", manifestRel)
	return Result{
		Manifest:     manifest,
		TotalFiles:   len(files),
		NewObjects:   newObjects,
		TotalBytes:   totalBytes,
		ManifestPath: manifestRel,
	}, nil
}

// hashAndStore hashes src while copying it into the CAS at root/<ab>/<sha>.
// One file read, one write — SHA in the manifest is guaranteed to match the
// stored bytes. Returns wrote=false when the object already existed (dedup).
func hashAndStore(root, src string) (sha string, size int64, wrote bool, err error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, false, err
	}
	defer in.Close()

	// Capture source permissions before any copying so CAS objects on Linux/macOS
	// preserve the execute bit. Without this, java/javaw ends up 0o600 and
	// exec.Command fails with EACCES on the first launch.
	srcInfo, err := in.Stat()
	if err != nil {
		return "", 0, false, err
	}
	srcMode := srcInfo.Mode()

	tmp, err := os.CreateTemp(root, "tmp-*")
	if err != nil {
		return "", 0, false, err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		tmp.Close()
		if !committed {
			os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), in)
	if err != nil {
		return "", 0, false, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, false, err
	}

	sha = hex.EncodeToString(h.Sum(nil))
	size = n
	dst := filepath.Join(root, sha[:2], sha)

	if _, serr := os.Stat(dst); serr == nil {
		// Idempotent: update permissions on objects baked before the fix was added.
		_ = os.Chmod(dst, srcMode)
		return sha, size, false, nil // already in CAS — dedup
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, false, err
	}
	// Apply source permissions before rename so the CAS object has the right mode.
	// On Windows this is a no-op (only the read-only bit matters there).
	if err := os.Chmod(tmpName, srcMode); err != nil {
		return "", 0, false, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", 0, false, err
	}
	committed = true
	return sha, size, true, nil
}

func matchOptional(relSlash string, rules []OptionalRule) (bool, string) {
	for _, r := range rules {
		if ok, _ := path.Match(r.Glob, relSlash); ok {
			return true, r.ID
		}
	}
	return false, ""
}

func writeManifest(dst string, m engine.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(dst), "tmp-manifest-*")
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
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	committed = true
	return nil
}
