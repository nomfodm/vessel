package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nomfodm/vessel/internal/paths"
)

// mapFetcher serves object bytes from an in-memory map keyed by sha.
// It honours the offset argument so resume logic can be tested without a server.
type mapFetcher struct {
	objects map[string][]byte
	calls   int
}

func (m *mapFetcher) Fetch(_ context.Context, sha string, offset int64) (io.ReadCloser, int64, error) {
	m.calls++
	b, ok := m.objects[sha]
	if !ok {
		return nil, 0, os.ErrNotExist
	}
	if offset > int64(len(b)) {
		offset = int64(len(b))
	}
	return io.NopCloser(strings.NewReader(string(b[offset:]))), offset, nil
}

func newFetcher(contents ...string) (*mapFetcher, map[string]string) {
	f := &mapFetcher{objects: map[string][]byte{}}
	shaByContent := map[string]string{}
	for _, c := range contents {
		sha := sha256hex([]byte(c))
		f.objects[sha] = []byte(c)
		shaByContent[c] = sha
	}
	return f, shaByContent
}

func newTestEngine(t *testing.T) (*Engine, paths.Layout) {
	t.Helper()
	layout := paths.Resolve(t.TempDir())
	e, err := New(layout, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e, layout
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestSyncRequiredAndOptional(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("jei-jar", "map-jar", "optifine-jar")

	m := Manifest{
		Files: []ManifestFile{
			{Path: "mods/jei.jar", SHA256: sha["jei-jar"]},
			{Path: "mods/map.jar", SHA256: sha["map-jar"], Optional: true, ID: "minimap"},
			{Path: "mods/optifine.jar", SHA256: sha["optifine-jar"], Optional: true, ID: "optifine"},
		},
	}

	if err := e.Sync(context.Background(), m, "test", []string{"optifine"}, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := read(t, filepath.Join(layout.Root, "mods", "jei.jar")); got != "jei-jar" {
		t.Errorf("jei = %q", got)
	}
	if got := read(t, filepath.Join(layout.Root, "mods", "optifine.jar")); got != "optifine-jar" {
		t.Errorf("optifine = %q", got)
	}
	if _, err := os.Stat(filepath.Join(layout.Root, "mods", "map.jar")); !os.IsNotExist(err) {
		t.Error("disabled optional file was installed")
	}
}

func TestSyncProgress(t *testing.T) {
	e, _ := newTestEngine(t)
	fetch, sha := newFetcher("a", "b")
	m := Manifest{Files: []ManifestFile{
		{Path: "a.txt", SHA256: sha["a"], Size: int64(len("a"))},
		{Path: "b.txt", SHA256: sha["b"], Size: int64(len("b"))},
	}}

	var lastDone, lastTotal int64
	var callCount int
	cb := func(done, total int64, _ string) {
		lastDone, lastTotal = done, total
		callCount++
	}
	if err := e.Sync(context.Background(), m, "test", nil, fetch, cb); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if lastDone != lastTotal {
		t.Errorf("final progress = %d/%d bytes, want equal", lastDone, lastTotal)
	}
	if callCount == 0 {
		t.Error("no progress callbacks received")
	}
}

func TestSyncSkipsUpToDate(t *testing.T) {
	e, _ := newTestEngine(t)
	fetch, sha := newFetcher("payload")
	m := Manifest{Files: []ManifestFile{{Path: "x.bin", SHA256: sha["payload"]}}}

	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	first := fetch.calls
	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if fetch.calls != first {
		t.Errorf("second sync refetched: calls %d -> %d", first, fetch.calls)
	}
}

func TestSyncStrictDirPrunesForeign(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("good-mod")
	m := Manifest{
		Files:      []ManifestFile{{Path: "mods/good.jar", SHA256: sha["good-mod"]}},
		StrictDirs: []string{"mods"},
	}

	cheat := filepath.Join(layout.Root, "mods", "cheat.jar")
	writeFile(t, cheat, "x-ray")
	save := filepath.Join(layout.Root, "saves", "world", "level.dat")
	writeFile(t, save, "my world")

	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if _, err := os.Stat(cheat); !os.IsNotExist(err) {
		t.Error("foreign file in strict dir was not pruned")
	}
	if got := read(t, filepath.Join(layout.Root, "mods", "good.jar")); got != "good-mod" {
		t.Errorf("good mod missing/wrong: %q", got)
	}
	if got := read(t, save); got != "my world" {
		t.Error("user file outside strict dir was touched")
	}
}

func TestSyncResumesInterruptedDownload(t *testing.T) {
	e, layout := newTestEngine(t)
	content := "big-payload-content"
	sha := sha256hex([]byte(content))

	// Simulate an interrupted download: write the first half as a .part file.
	half := len(content) / 2
	partPath := filepath.Join(layout.ObjectsDir(), sha[:2], sha+".part")
	writeFile(t, partPath, content[:half])

	fetch, _ := newFetcher(content)
	m := Manifest{Files: []ManifestFile{{Path: "mods/big.jar", SHA256: sha, Size: int64(len(content))}}}

	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Fetcher should have been called once, serving only the second half.
	if fetch.calls != 1 {
		t.Errorf("fetch calls = %d, want 1", fetch.calls)
	}
	if got := read(t, filepath.Join(layout.Root, "mods", "big.jar")); got != content {
		t.Errorf("file content = %q, want %q", got, content)
	}
	// .part must be gone after a successful sync.
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Error(".part file was not cleaned up after successful sync")
	}
}

func TestSyncReplacesWrongHash(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("correct")
	m := Manifest{Files: []ManifestFile{{Path: "mods/m.jar", SHA256: sha["correct"]}}}

	dst := filepath.Join(layout.Root, "mods", "m.jar")
	writeFile(t, dst, "tampered")

	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := read(t, dst); got != "correct" {
		t.Errorf("file with wrong hash not replaced: %q", got)
	}
}

// TestSyncNilEnabledUsesDefaultOn verifies that a nil enabledOptional slice
// (first launch, never saved to config) falls back to each group's DefaultOn
// rather than skipping all optional files.
func TestSyncNilEnabledUsesDefaultOn(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("required", "default-on-mod", "opt-out-mod")

	m := Manifest{
		Files: []ManifestFile{
			{Path: "client.jar", SHA256: sha["required"]},
			{Path: "mods/on.jar", SHA256: sha["default-on-mod"], Optional: true, ID: "on"},
			{Path: "mods/off.jar", SHA256: sha["opt-out-mod"], Optional: true, ID: "off"},
		},
		Optional: []OptionalGroup{
			{ID: "on", DefaultOn: true},
			{ID: "off", DefaultOn: false},
		},
	}

	if err := e.Sync(context.Background(), m, "test", nil, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := read(t, filepath.Join(layout.Root, "client.jar")); got != "required" {
		t.Errorf("required file = %q", got)
	}
	if got := read(t, filepath.Join(layout.Root, "mods", "on.jar")); got != "default-on-mod" {
		t.Errorf("defaultOn mod should be installed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(layout.Root, "mods", "off.jar")); !os.IsNotExist(err) {
		t.Error("defaultOff mod should not be installed with nil enabledOptional")
	}
}

// TestSyncEmptyEnabledSkipsAll verifies that an explicit empty slice (user
// deliberately disabled everything) is distinct from nil and skips all optionals.
func TestSyncEmptyEnabledSkipsAll(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("required", "default-on-mod")

	m := Manifest{
		Files: []ManifestFile{
			{Path: "client.jar", SHA256: sha["required"]},
			{Path: "mods/on.jar", SHA256: sha["default-on-mod"], Optional: true, ID: "on"},
		},
		Optional: []OptionalGroup{{ID: "on", DefaultOn: true}},
	}

	if err := e.Sync(context.Background(), m, "test", []string{}, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if _, err := os.Stat(filepath.Join(layout.Root, "mods", "on.jar")); !os.IsNotExist(err) {
		t.Error("empty enabledOptional should skip all optionals, even defaultOn ones")
	}
}
