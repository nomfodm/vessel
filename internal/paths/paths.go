// Package paths resolves per-OS locations for vessel's config, data, cache and
// logs. Functions return clean strings and never touch the filesystem.
package paths

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

const appName = "Infinity"

// ConfigFile is at a fixed OS location (not under dataRoot): it holds dataRoot,
// so it must be resolvable before anything else and must not move with the data.
func ConfigFile() string {
	return filepath.Join(xdg.ConfigHome, appName, "config.json")
}

func DefaultDataRoot() string {
	return filepath.Join(xdg.DataHome, appName)
}

func CacheDir() string {
	return filepath.Join(xdg.CacheHome, appName)
}

func LogDir() string {
	return filepath.Join(xdg.StateHome, appName, "logs")
}

type Layout struct {
	Root string
}

func Resolve(override string) Layout {
	root := override
	if root == "" {
		root = DefaultDataRoot()
	}
	return Layout{Root: root}
}

func (l Layout) ObjectsDir() string {
	return filepath.Join(l.Root, "objects")
}

func (l Layout) RuntimesDir() string {
	return filepath.Join(l.Root, "runtimes")
}

func (l Layout) ProfileDir(slug string) string {
	return filepath.Join(l.Root, "profiles", slug)
}
