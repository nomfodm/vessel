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
// outDir/profiles/<slug>/<manifestName>. specSkip is the spec filename to ignore
// when it lives inside clientDir.
func Bake(log *slog.Logger, clientDir, slug, outDir, manifestName, specSkip string, spec Spec) (Result, error) {
	if log == nil {
		log = slog.Default()
	}
	log = log.With("cmd", "baker")

	filesDir := filepath.Join(outDir, "files")
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

		sha, size, err := hashFile(p)
		if err != nil {
			return fmt.Errorf("hash %s: %w", relSlash, err)
		}

		optional, id := matchOptional(relSlash, spec.Optional)
		if optional {
			matched[id] = true
		}
		files = append(files, engine.ManifestFile{
			Path:     relSlash,
			SHA256:   sha,
			Size:     size,
			Optional: optional,
			ID:       id,
		})

		wrote, err := storeObject(filesDir, sha, p)
		if err != nil {
			return fmt.Errorf("store %s: %w", relSlash, err)
		}
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

	manifest := engine.Manifest{
		Files:            files,
		Optional:         groups,
		StrictDirs:       spec.StrictDirs,
		Runtime:          spec.Runtime,
		RecommendedRAMMB: spec.RecommendedRamMB,
		MinRAMMB:         spec.MinRamMB,
	}

	relManifest := path.Join("profiles", slug, manifestName)
	if err := writeManifest(filepath.Join(outDir, filepath.FromSlash(relManifest)), manifest); err != nil {
		return Result{}, err
	}

	log.Info("bake done", "slug", slug, "files", len(files), "newObjects", newObjects, "manifest", relManifest)
	return Result{
		Manifest:     manifest,
		TotalFiles:   len(files),
		NewObjects:   newObjects,
		TotalBytes:   totalBytes,
		ManifestPath: relManifest,
	}, nil
}

func matchOptional(relSlash string, rules []OptionalRule) (bool, string) {
	for _, r := range rules {
		if ok, _ := path.Match(r.Glob, relSlash); ok {
			return true, r.ID
		}
	}
	return false, ""
}

func hashFile(p string) (string, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// storeObject copies src into the CAS at <root>/<ab>/<sha>, returning false when
// the object already exists (dedup). The copy is atomic via a temp file rename.
func storeObject(root, sha, src string) (bool, error) {
	dst := filepath.Join(root, sha[:2], sha)
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}

	in, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), "tmp-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		tmp.Close()
		if !renamed {
			os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return false, err
	}
	renamed = true
	return true, nil
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
	return os.WriteFile(dst, data, 0o644)
}
