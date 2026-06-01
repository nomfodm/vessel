package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nomfodm/vessel/internal/engine"
	"github.com/nomfodm/vessel/internal/paths"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// s3 serves a fixed set of paths, like a static bucket.
func s3(files map[string]string) *httptest.Server {
	mux := http.NewServeMux()
	for path, body := range files {
		b := body
		mux.HandleFunc("/"+path, func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, b)
		})
	}
	return httptest.NewServer(mux)
}

const indexJSON = `{
  "version": 3,
  "profiles": [
    {"slug": "survival", "title": "Выживание", "manifestPath": "profiles/survival/files-v5.json"}
  ]
}`

func TestProfiles(t *testing.T) {
	srv := s3(map[string]string{"index.json": indexJSON})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	got, err := svc.Profiles(context.Background())
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "survival" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Title != "Выживание" {
		t.Errorf("title = %q", got[0].Title)
	}
}

func TestManifest(t *testing.T) {
	manifest := `{"files":[{"path":"mods/jei.jar","sha256":"abc","size":10}],"strictDirs":["mods"],"runtime":"jre-21"}`
	srv := s3(map[string]string{
		"index.json":                      indexJSON,
		"profiles/survival/files-v5.json": manifest,
	})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	m, err := svc.Manifest(context.Background(), "survival")
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if len(m.Files) != 1 || m.Files[0].Path != "mods/jei.jar" {
		t.Fatalf("files = %+v", m.Files)
	}
	if m.Runtime != "jre-21" {
		t.Errorf("runtime = %q", m.Runtime)
	}
}

const indexRich = `{
  "version": 4,
  "profiles": [
    {"slug": "b", "title": "B", "order": 2},
    {"slug": "a", "title": "A", "order": 1,
     "mcVersion": "1.20.1",
     "servers": [
       {"name": "Main", "host": "play.infinityserver.ru", "port": 25565},
       {"name": "Events", "host": "events.infinityserver.ru", "port": 25566}
     ]},
    {"slug": "secret", "title": "Secret", "order": 0, "hidden": true,
     "manifestPath": "profiles/secret/files-v1.json"}
  ]
}`

func TestProfilesFiltersHiddenAndSorts(t *testing.T) {
	srv := s3(map[string]string{"index.json": indexRich})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	got, err := svc.Profiles(context.Background())
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d profiles, want 2 (hidden excluded)", len(got))
	}
	if got[0].Slug != "a" || got[1].Slug != "b" {
		t.Errorf("order = [%s %s], want [a b]", got[0].Slug, got[1].Slug)
	}
}

func TestProfileServers(t *testing.T) {
	srv := s3(map[string]string{"index.json": indexRich})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	got, _ := svc.Profiles(context.Background())

	var a Profile
	for _, p := range got {
		if p.Slug == "a" {
			a = p
		}
	}
	if a.MCVersion != "1.20.1" {
		t.Errorf("mcVersion = %q", a.MCVersion)
	}
	if len(a.Servers) != 2 {
		t.Fatalf("servers = %d, want 2", len(a.Servers))
	}
	if a.Servers[0].Host != "play.infinityserver.ru" || a.Servers[0].Port != 25565 {
		t.Errorf("server[0] = %+v", a.Servers[0])
	}
}

func TestManifestResolvesHiddenProfile(t *testing.T) {
	manifest := `{"files":[],"recommendedRamMB":4096,"minRamMB":2048}`
	srv := s3(map[string]string{
		"index.json":                    indexRich,
		"profiles/secret/files-v1.json": manifest,
	})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	m, err := svc.Manifest(context.Background(), "secret")
	if err != nil {
		t.Fatalf("hidden profile manifest should resolve: %v", err)
	}
	if m.RecommendedRAMMB != 4096 || m.MinRAMMB != 2048 {
		t.Errorf("ram fields = %d/%d", m.RecommendedRAMMB, m.MinRAMMB)
	}
}

func TestManifestUnknownProfile(t *testing.T) {
	srv := s3(map[string]string{"index.json": indexJSON})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	if _, err := svc.Manifest(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestFetcherShardedPath(t *testing.T) {
	content := "a jar file"
	sha := sha256hex([]byte(content))
	srv := s3(map[string]string{"files/" + sha[:2] + "/" + sha: content})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	rc, err := svc.Fetcher().Fetch(context.Background(), sha)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("fetched %q, want %q", got, content)
	}
}

// End-to-end: ProfileService.Fetcher feeds the engine over real HTTP.
func TestFetcherDrivesEngineSync(t *testing.T) {
	jar := "jei-bytes"
	sha := sha256hex([]byte(jar))
	manifest := `{"files":[{"path":"mods/jei.jar","sha256":"` + sha + `"}]}`
	srv := s3(map[string]string{
		"index.json":                      indexJSON,
		"profiles/survival/files-v5.json": manifest,
		"files/" + sha[:2] + "/" + sha:    jar,
	})
	defer srv.Close()

	svc := New(srv.URL, srv.Client(), testLogger())
	m, err := svc.Manifest(context.Background(), "survival")
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}

	layout := paths.Resolve(t.TempDir())
	e, err := engine.New(layout, testLogger())
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	if err := e.Sync(context.Background(), m, nil, svc.Fetcher(), nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func TestFetchMissingObject(t *testing.T) {
	srv := s3(map[string]string{})
	defer srv.Close()
	svc := New(srv.URL, srv.Client(), testLogger())
	if _, err := svc.Fetcher().Fetch(context.Background(), sha256hex([]byte("x"))); err == nil {
		t.Fatal("expected error for missing object")
	}
}
