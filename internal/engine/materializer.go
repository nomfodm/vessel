package engine

import (
	"io"
	"os"
	"path/filepath"
)

type Materializer interface {
	Materialize(src, dst string) error
}

type linkMaterializer struct{}

func (linkMaterializer) Materialize(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	os.Remove(tmp)
	if err := os.Link(src, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

type copyMaterializer struct{}

func (copyMaterializer) Materialize(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// canHardlink probes whether dir's volume supports hardlinks. Hardlinks fail
// across volumes and on filesystems like exFAT, so we test once and fall back to
// copying.
func canHardlink(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	src, err := os.CreateTemp(dir, "probe-*")
	if err != nil {
		return false
	}
	srcName := src.Name()
	src.Close()
	defer os.Remove(srcName)

	link := srcName + ".link"
	if err := os.Link(srcName, link); err != nil {
		return false
	}
	os.Remove(link)
	return true
}
