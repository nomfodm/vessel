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

// DataDirs are the subdirectories under Root that hold relocatable game data.
// Anything else that may live under Root (on Windows the config file and logs
// share %LOCALAPPDATA%\Infinity) is NOT data and must not be moved — notably the
// open log file, which can't be renamed on Windows.
func (l Layout) DataDirs() []string {
	return []string{"objects", "profiles", "runtimes", ".sync-states"}
}
