package system

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/nomfodm/vessel/internal/paths"
)

func TestTotalRAMMB(t *testing.T) {
	s := New(paths.Resolve(t.TempDir()), nil, nil, nil)
	if got := s.TotalRAMMB(); got <= 0 {
		t.Fatalf("TotalRAMMB = %d, want > 0", got)
	}
}

func TestProfileDir(t *testing.T) {
	root := t.TempDir()
	s := New(paths.Resolve(root), nil, nil, nil)
	want := filepath.Join(root, "profiles", "survival")
	if got := s.ProfileDir("survival"); got != want {
		t.Fatalf("ProfileDir = %q, want %q", got, want)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

var dataDirs = []string{"objects", "profiles", "runtimes"}

func TestRelocateSameVolumeMovesOnlyData(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	write(t, filepath.Join(old, "objects", "ab", "obj"), "data")
	write(t, filepath.Join(old, "profiles", "survival", "mod.jar"), "mod")
	write(t, filepath.Join(old, "logs", "app.log"), "log") // must stay (open on Windows)
	write(t, filepath.Join(old, "config.json"), "{}")      // must stay (fixed path)

	// t.TempDir keeps both on the same volume, so relocate takes the rename path.
	if err := relocate(old, newRoot, dataDirs, []string{"objects"}, testLogger()); err != nil {
		t.Fatalf("relocate: %v", err)
	}

	if got := readFile(t, filepath.Join(newRoot, "objects", "ab", "obj")); got != "data" {
		t.Errorf("object not moved: %q", got)
	}
	if got := readFile(t, filepath.Join(newRoot, "profiles", "survival", "mod.jar")); got != "mod" {
		t.Errorf("profile not moved (same-volume keeps everything): %q", got)
	}
	// The crux: logs and config are NOT data — they stay put.
	if got := readFile(t, filepath.Join(old, "logs", "app.log")); got != "log" {
		t.Error("logs were moved — the open log file would break this")
	}
	if got := readFile(t, filepath.Join(old, "config.json")); got != "{}" {
		t.Error("config.json was moved")
	}
	// Old root survives because logs/config remain; moved data is gone from it.
	if _, err := os.Stat(old); err != nil {
		t.Error("old root removed despite remaining logs/config")
	}
	if _, err := os.Stat(filepath.Join(old, "objects")); !os.IsNotExist(err) {
		t.Error("old objects not removed after move")
	}
}

func TestRelocateCrossVolumeKeepsObjectsDropsRest(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	write(t, filepath.Join(old, "objects", "ab", "obj"), "data")
	write(t, filepath.Join(old, "profiles", "survival", "mod.jar"), "mod")
	write(t, filepath.Join(old, "logs", "app.log"), "log")

	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	// Directly exercise the cross-volume strategy: keep objects, drop the rest.
	if err := copyKeepDropDirs(old, newRoot, dataDirs, []string{"objects"}); err != nil {
		t.Fatalf("copyKeepDropDirs: %v", err)
	}

	if got := readFile(t, filepath.Join(newRoot, "objects", "ab", "obj")); got != "data" {
		t.Errorf("object not copied: %q", got)
	}
	if _, err := os.Stat(filepath.Join(newRoot, "profiles")); !os.IsNotExist(err) {
		t.Error("profiles should be dropped cross-volume (re-materializes on launch)")
	}
	if _, err := os.Stat(filepath.Join(old, "objects")); !os.IsNotExist(err) {
		t.Error("old objects not removed")
	}
	if got := readFile(t, filepath.Join(old, "logs", "app.log")); got != "log" {
		t.Error("logs touched cross-volume")
	}
}

func TestValidateNewRoot(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "old")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := validateNewRoot(old, old, dataDirs); err == nil {
		t.Error("same path should be rejected")
	}
	if err := validateNewRoot(old, filepath.Join(old, "sub"), dataDirs); err == nil {
		t.Error("nested path should be rejected")
	}

	// A folder with only unrelated files (like the default root's config/logs) is
	// fine — this is what makes "return to default" work.
	withConfig := filepath.Join(base, "withconfig")
	write(t, filepath.Join(withConfig, "config.json"), "{}")
	write(t, filepath.Join(withConfig, "logs", "app.log"), "log")
	if err := validateNewRoot(old, withConfig, dataDirs); err != nil {
		t.Errorf("folder with only config/logs should pass: %v", err)
	}

	// A folder already holding game data must be rejected (would collide).
	withData := filepath.Join(base, "withdata")
	write(t, filepath.Join(withData, "objects", "x"), "y")
	if err := validateNewRoot(old, withData, dataDirs); err == nil {
		t.Error("folder containing objects/ should be rejected")
	}

	empty := filepath.Join(base, "empty")
	if err := validateNewRoot(old, empty, dataDirs); err != nil {
		t.Errorf("empty (creatable) target should pass: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
