package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvedPaths(t *testing.T) {
	t.Logf("ConfigFile      = %s", ConfigFile())
	t.Logf("DefaultDataRoot = %s", DefaultDataRoot())
	t.Logf("CacheDir        = %s", CacheDir())
	t.Logf("LogDir          = %s", LogDir())

	if !strings.HasSuffix(ConfigFile(), filepath.Join(appName, "config.json")) {
		t.Errorf("ConfigFile() = %q, want it to end with %q", ConfigFile(), filepath.Join(appName, "config.json"))
	}

	if CacheDir() == DefaultDataRoot() {
		t.Errorf("CacheDir() == DefaultDataRoot() == %q; cache must be a distinct location", CacheDir())
	}

	if got := Resolve("").Root; got != DefaultDataRoot() {
		t.Errorf("Resolve(\"\").Root = %q, want %q", got, DefaultDataRoot())
	}

	custom := Resolve(`D:\games\infinity`)
	t.Logf("Layout(custom).ObjectsDir  = %s", custom.ObjectsDir())
	t.Logf("Layout(custom).RuntimesDir = %s", custom.RuntimesDir())
	t.Logf("Layout(custom).ProfileDir  = %s", custom.ProfileDir("survival"))

	if custom.Root != `D:\games\infinity` {
		t.Errorf("override ignored: Root = %q", custom.Root)
	}
	if got, want := custom.ProfileDir("survival"), filepath.Join(`D:\games\infinity`, "profiles", "survival"); got != want {
		t.Errorf("ProfileDir = %q, want %q", got, want)
	}
}
