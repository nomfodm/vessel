package health

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func apiServer(t *testing.T, status string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"status":%q}`, status)
	}))
}

func TestReachableAndOperational(t *testing.T) {
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer storage.Close()
	api := apiServer(t, "operational")
	defer api.Close()

	svc := New([]Target{
		{Name: "storage", URL: storage.URL, Kind: Reachable},
		{Name: "api", URL: api.URL, Kind: APIHealth},
	}, nil, nil)
	rep := svc.Check(context.Background())
	if !rep.OK {
		t.Fatalf("expected OK, got %+v", rep)
	}
}

func TestMaintenanceNotOK(t *testing.T) {
	api := apiServer(t, "maintenance")
	defer api.Close()

	svc := New([]Target{{Name: "api", URL: api.URL, Kind: APIHealth}}, nil, nil)
	rep := svc.Check(context.Background())
	if rep.OK {
		t.Fatal("maintenance must not be OK")
	}
	if rep.Statuses[0].State != "maintenance" {
		t.Fatalf("state = %q, want maintenance", rep.Statuses[0].State)
	}
}

func TestOffNotOK(t *testing.T) {
	api := apiServer(t, "off")
	defer api.Close()

	svc := New([]Target{{Name: "api", URL: api.URL, Kind: APIHealth}}, nil, nil)
	rep := svc.Check(context.Background())
	if rep.OK || rep.Statuses[0].State != "off" {
		t.Fatalf("want off/not-ok, got %+v", rep.Statuses[0])
	}
}

func TestAPIUnreachable(t *testing.T) {
	svc := New([]Target{{Name: "api", URL: "http://127.0.0.1:0/health", Kind: APIHealth}}, nil, nil)
	rep := svc.Check(context.Background())
	if rep.OK || rep.Statuses[0].State != "unreachable" {
		t.Fatalf("want unreachable/not-ok, got %+v", rep.Statuses[0])
	}
}

func TestReachableServerError(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	svc := New([]Target{{Name: "storage", URL: down.URL, Kind: Reachable}}, nil, nil)
	rep := svc.Check(context.Background())
	if rep.OK {
		t.Fatal("expected not-OK for 5xx")
	}
}
