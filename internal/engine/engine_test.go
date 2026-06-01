package engine

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nomfodm/vessel/internal/paths"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewCreatesObjectsDir(t *testing.T) {
	layout := paths.Resolve(t.TempDir())
	e, err := New(layout, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(layout.ObjectsDir()); err != nil {
		t.Errorf("objects dir not created: %v", err)
	}
	if e.mat == nil {
		t.Error("materializer not selected")
	}
}

func TestEngineMaterialize(t *testing.T) {
	layout := paths.Resolve(t.TempDir())
	e, err := New(layout, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content := "a mod jar"
	sha := sha256hex([]byte(content))
	if err := e.store.Add(strings.NewReader(content), sha); err != nil {
		t.Fatalf("store.Add: %v", err)
	}

	dst := filepath.Join(layout.ProfileDir("survival"), "mods", "cool.jar")
	if err := e.materialize(sha, dst); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read materialized file: %v", err)
	}
	if string(got) != content {
		t.Errorf("materialized content = %q, want %q", got, content)
	}
}
