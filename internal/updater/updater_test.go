package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckNeedsUpdate(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{
			"needs_update": true,
			"latest": {
				"id": 7, "version": "1.5.0", "platform": "win",
				"download_url": "https://dl/launcher.exe", "checksum": "abc",
				"file_size": 1024, "changelog": ["fix a", "fix b"],
				"released_at": "2026-01-01T00:00:00Z", "is_mandatory": true
			}
		}`))
	}))
	defer srv.Close()

	info, err := New("1.0.0", srv.URL, nil, nil, nil).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.NeedsUpdate || info.Latest != "1.5.0" || info.DownloadURL != "https://dl/launcher.exe" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.Checksum != "abc" || info.FileSize != 1024 || !info.IsMandatory || len(info.Changelog) != 2 {
		t.Fatalf("release fields not mapped: %+v", info)
	}
	if gotQuery != "platform="+platform()+"&version=1.0.0" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestCheckUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"needs_update": false, "latest": null}`))
	}))
	defer srv.Close()

	info, err := New("1.0.0", srv.URL, nil, nil, nil).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.NeedsUpdate || info.Latest != "" {
		t.Fatalf("should be up to date: %+v", info)
	}
}

func TestCheckDevSkips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("dev build must not call the API")
	}))
	defer srv.Close()

	info, err := New("dev", srv.URL, nil, nil, nil).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.NeedsUpdate {
		t.Fatalf("dev should never need update: %+v", info)
	}
}
