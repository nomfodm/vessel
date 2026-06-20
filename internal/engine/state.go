package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type fileState struct {
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
	SHA   string    `json:"sha"`
}

type SyncState struct {
	ManifestSHA string               `json:"manifestSHA"`
	Files       map[string]fileState `json:"files"`
}

func loadState(path string) SyncState {
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncState{Files: make(map[string]fileState)}
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		return SyncState{Files: make(map[string]fileState)}
	}
	if s.Files == nil {
		s.Files = make(map[string]fileState)
	}
	return s
}

func saveState(path string, s SyncState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// manifestFingerprint hashes path+sha of all desired files in sorted order.
// Changes when any file is added, removed, or gets a new hash.
func manifestFingerprint(files []ManifestFile) string {
	sorted := make([]ManifestFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path + ":" + f.SHA256 + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
