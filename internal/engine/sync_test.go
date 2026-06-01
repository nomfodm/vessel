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
type mapFetcher struct {
	objects map[string][]byte
	calls   int
}

func (m *mapFetcher) Fetch(_ context.Context, sha string) (io.ReadCloser, error) {
	m.calls++
	b, ok := m.objects[sha]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(string(b))), nil
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

	if err := e.Sync(context.Background(), m, []string{"optifine"}, fetch, nil); err != nil {
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
		{Path: "a.txt", SHA256: sha["a"]},
		{Path: "b.txt", SHA256: sha["b"]},
	}}

	var lastDone, lastTotal int
	var seen []string
	cb := func(done, total int, file string) {
		lastDone, lastTotal = done, total
		seen = append(seen, file)
	}
	if err := e.Sync(context.Background(), m, nil, fetch, cb); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if lastDone != 2 || lastTotal != 2 {
		t.Errorf("final progress = %d/%d, want 2/2", lastDone, lastTotal)
	}
	if len(seen) != 2 {
		t.Errorf("progress callbacks = %d, want 2", len(seen))
	}
}

func TestSyncSkipsUpToDate(t *testing.T) {
	e, _ := newTestEngine(t)
	fetch, sha := newFetcher("payload")
	m := Manifest{Files: []ManifestFile{{Path: "x.bin", SHA256: sha["payload"]}}}

	if err := e.Sync(context.Background(), m, nil, fetch, nil); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	first := fetch.calls
	if err := e.Sync(context.Background(), m, nil, fetch, nil); err != nil {
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

	if err := e.Sync(context.Background(), m, nil, fetch, nil); err != nil {
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

func TestSyncReplacesWrongHash(t *testing.T) {
	e, layout := newTestEngine(t)
	fetch, sha := newFetcher("correct")
	m := Manifest{Files: []ManifestFile{{Path: "mods/m.jar", SHA256: sha["correct"]}}}

	dst := filepath.Join(layout.Root, "mods", "m.jar")
	writeFile(t, dst, "tampered")

	if err := e.Sync(context.Background(), m, nil, fetch, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := read(t, dst); got != "correct" {
		t.Errorf("file with wrong hash not replaced: %q", got)
	}
}
