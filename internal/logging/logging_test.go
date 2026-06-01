package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesToFile(t *testing.T) {
	dir := t.TempDir()

	logger, closer, err := New(dir, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("hello", "answer", 42)
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(dir, filePrefix+"*.log"))
	if len(files) != 1 {
		t.Fatalf("got %d log files, want 1", len(files))
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "hello") || !strings.Contains(body, "answer=42") {
		t.Errorf("log file missing expected content:\n%s", body)
	}
}

func TestPruneKeepsRecent(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"launcher-20240101-000001.log",
		"launcher-20240102-000001.log",
		"launcher-20240103-000001.log",
		"launcher-20240104-000001.log",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prune(dir, 2); err != nil {
		t.Fatalf("prune: %v", err)
	}

	left, _ := filepath.Glob(filepath.Join(dir, filePrefix+"*.log"))
	if len(left) != 2 {
		t.Fatalf("got %d files after prune, want 2", len(left))
	}
	for _, p := range left {
		base := filepath.Base(p)
		if base != "launcher-20240103-000001.log" && base != "launcher-20240104-000001.log" {
			t.Errorf("kept wrong file: %s", base)
		}
	}
}
