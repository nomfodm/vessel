package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyMaterializer(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "out", "nested", "dst")
	writeFile(t, src, "payload")

	if err := (copyMaterializer{}).Materialize(src, dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("dst content = %q, want payload", got)
	}
}

func TestLinkMaterializer(t *testing.T) {
	dir := t.TempDir()
	if !canHardlink(dir) {
		t.Skip("filesystem does not support hardlinks")
	}
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "out", "dst")
	writeFile(t, src, "linked")

	if err := (linkMaterializer{}).Materialize(src, dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	si, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	di, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(si, di) {
		t.Error("link materializer did not produce a hardlink (different files)")
	}
}

func TestCanHardlink(t *testing.T) {
	t.Logf("canHardlink(tempdir) = %v", canHardlink(t.TempDir()))
}
