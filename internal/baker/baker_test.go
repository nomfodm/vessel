package baker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nomfodm/vessel/internal/engine"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func TestBake(t *testing.T) {
	client := t.TempDir()
	out := t.TempDir()

	write(t, client, "mods/core.jar", "core")
	write(t, client, "mods/OptiFine_1.0.jar", "optifine")
	write(t, client, "config/options.txt", "opts")
	write(t, client, "dup.txt", "core")  // same content as core.jar -> dedup
	write(t, client, "baker.json", "{}") // must be skipped

	spec := Spec{
		Runtime:          "java17",
		RecommendedRamMB: 4096,
		MinRamMB:         2048,
		StrictDirs:       []string{"mods", "config"},
		Optional: []OptionalRule{
			{ID: "optifine", Glob: "mods/OptiFine*.jar", Name: "OptiFine HD", Desc: "FPS", DefaultOn: true},
			{ID: "ghost", Glob: "mods/Nonexistent*.jar", Name: "Ghost"}, // matches nothing
		},
	}

	res, err := Bake(nil, client, "profiles/survival", out, "profiles/survival/files-v1.json", "baker.json", spec)
	if err != nil {
		t.Fatal(err)
	}

	// 4 real files (baker.json skipped), but only 3 unique objects (core==dup).
	if res.TotalFiles != 4 {
		t.Fatalf("TotalFiles = %d, want 4", res.TotalFiles)
	}
	if res.NewObjects != 3 {
		t.Fatalf("NewObjects = %d, want 3 (dedup)", res.NewObjects)
	}

	// Files sorted by path, each prefixed with profiles/survival.
	got := res.Manifest.Files
	wantOrder := []string{
		"profiles/survival/config/options.txt",
		"profiles/survival/dup.txt",
		"profiles/survival/mods/OptiFine_1.0.jar",
		"profiles/survival/mods/core.jar",
	}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d files, want %d", len(got), len(wantOrder))
	}
	for i, p := range wantOrder {
		if got[i].Path != p {
			t.Fatalf("file[%d].Path = %q, want %q", i, got[i].Path, p)
		}
	}

	// OptiFine flagged optional with id; others not. (Glob matched the unprefixed
	// path, but the stored path is prefixed.)
	for _, f := range got {
		if f.Path == "profiles/survival/mods/OptiFine_1.0.jar" {
			if !f.Optional || f.ID != "optifine" {
				t.Fatalf("OptiFine optional=%v id=%q, want true/optifine", f.Optional, f.ID)
			}
		} else if f.Optional {
			t.Fatalf("%s unexpectedly optional", f.Path)
		}
	}

	// Strict dirs are prefixed too.
	wantStrict := []string{"profiles/survival/mods", "profiles/survival/config"}
	if len(res.Manifest.StrictDirs) != len(wantStrict) {
		t.Fatalf("StrictDirs = %v, want %v", res.Manifest.StrictDirs, wantStrict)
	}
	for i, sd := range wantStrict {
		if res.Manifest.StrictDirs[i] != sd {
			t.Fatalf("StrictDirs[%d] = %q, want %q", i, res.Manifest.StrictDirs[i], sd)
		}
	}

	// Optional groups: only the matched rule survives, with its metadata.
	if len(res.Manifest.Optional) != 1 {
		t.Fatalf("Optional groups = %d, want 1 (ghost dropped)", len(res.Manifest.Optional))
	}
	g := res.Manifest.Optional[0]
	if g.ID != "optifine" || g.Name != "OptiFine HD" || !g.DefaultOn {
		t.Fatalf("optional group wrong: %+v", g)
	}

	// Object present in CAS at files/<ab>/<sha> with correct hash-name.
	coreSha := sha("core")
	objPath := filepath.Join(out, "files", coreSha[:2], coreSha)
	if _, err := os.Stat(objPath); err != nil {
		t.Fatalf("CAS object missing: %v", err)
	}

	// baker.json never became an object.
	specSha := sha("{}")
	if _, err := os.Stat(filepath.Join(out, "files", specSha[:2], specSha)); err == nil {
		t.Fatal("baker.json was stored as an object")
	}

	// Manifest on disk round-trips into engine.Manifest.
	data, err := os.ReadFile(filepath.Join(out, "profiles", "survival", "files-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m engine.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest not valid engine.Manifest: %v", err)
	}
	if m.Runtime != "java17" || m.RecommendedRAMMB != 4096 || m.MinRAMMB != 2048 {
		t.Fatalf("manifest spec fields wrong: %+v", m)
	}
}

func TestBakeIdempotentDedup(t *testing.T) {
	client := t.TempDir()
	out := t.TempDir()
	write(t, client, "a.txt", "hello")

	if _, err := Bake(nil, client, "", out, "files-v1.json", "", Spec{}); err != nil {
		t.Fatal(err)
	}
	// Second run sees the object already there -> nothing new written.
	res, err := Bake(nil, client, "", out, "files-v1.json", "", Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if res.NewObjects != 0 {
		t.Fatalf("NewObjects = %d on re-bake, want 0", res.NewObjects)
	}
}

func TestBakeRuntimePrefix(t *testing.T) {
	client := t.TempDir()
	out := t.TempDir()
	write(t, client, "bin/javaw.exe", "jvm")
	write(t, client, "lib/rt.jar", "rt")

	res, err := Bake(nil, client, "runtimes/java17", out, "runtimes/java17/win-x64/files-v1.json", "",
		Spec{StrictDirs: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"runtimes/java17/bin/javaw.exe", "runtimes/java17/lib/rt.jar"}
	if len(res.Manifest.Files) != len(want) {
		t.Fatalf("got %d files, want %d", len(res.Manifest.Files), len(want))
	}
	for i, p := range want {
		if res.Manifest.Files[i].Path != p {
			t.Fatalf("file[%d].Path = %q, want %q", i, res.Manifest.Files[i].Path, p)
		}
	}
	// "." strict dir resolves to the runtime root.
	if len(res.Manifest.StrictDirs) != 1 || res.Manifest.StrictDirs[0] != "runtimes/java17" {
		t.Fatalf("StrictDirs = %v, want [runtimes/java17]", res.Manifest.StrictDirs)
	}
	// Manifest landed at the platform-specific path.
	if _, err := os.Stat(filepath.Join(out, "runtimes", "java17", "win-x64", "files-v1.json")); err != nil {
		t.Fatalf("runtime manifest missing: %v", err)
	}
}

func TestLoadSpecBadGlob(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "baker.json")
	os.WriteFile(p, []byte(`{"optional":[{"id":"x","glob":"["}]}`), 0o644)
	if _, err := LoadSpec(p); err == nil {
		t.Fatal("expected error for bad glob")
	}
}
