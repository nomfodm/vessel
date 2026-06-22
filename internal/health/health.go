package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Kind selects how a target is probed.
type Kind string

const (
	// Reachable: up when the HTTP roundtrip returns a status below 500. A 4xx
	// still proves the host is alive — only transport failures and 5xx are down.
	Reachable Kind = "reachable"
	// Reachable2xx: up only when the HTTP roundtrip returns a 2xx status.
	// Use for object-storage endpoints where 403/404 means the service is broken.
	Reachable2xx Kind = "reachable2xx"
	// APIHealth: GET the URL and read {"status": "..."} — only "operational"
	// counts as up; "maintenance"/"off" are reported distinctly.
	APIHealth Kind = "apiHealth"
)

// Target is one server the launcher must reach before it is usable.
type Target struct {
	Name string
	URL  string
	Kind Kind
}

type Status struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	// State is a machine-readable detail: "operational", "maintenance", "off",
	// "unreachable" or "error". The UI branches on it.
	State string `json:"state"`
}

type Report struct {
	OK       bool     `json:"ok"`
	Statuses []Status `json:"statuses"`
}

type Service struct {
	targets []Target
	http    *http.Client
	log     *slog.Logger
}

func New(targets []Target, client *http.Client, log *slog.Logger) *Service {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{targets: targets, http: client, log: log.With("svc", "health")}
}

// Check probes every target concurrently. The report is OK only when every
// target is up.
func (s *Service) Check(ctx context.Context) Report {
	statuses := make([]Status, len(s.targets))
	var wg sync.WaitGroup
	for i, t := range s.targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			statuses[i] = s.probe(ctx, t)
		}(i, t)
	}
	wg.Wait()

	report := Report{OK: true, Statuses: statuses}
	for _, st := range statuses {
		if !st.OK {
			report.OK = false
		}
	}
	if report.OK {
		s.log.Info("connectivity check ok")
	} else {
		s.log.Warn("connectivity check failed", "statuses", statuses)
	}
	return report
}

func (s *Service) probe(ctx context.Context, t Target) Status {
	switch t.Kind {
	case APIHealth:
		return s.probeAPIHealth(ctx, t)
	case Reachable2xx:
		return s.probeReachable2xx(ctx, t)
	default:
		return s.probeReachable(ctx, t)
	}
}

func (s *Service) probeReachable(ctx context.Context, t Target) Status {
	st := Status{Name: t.Name, State: "unreachable"}
	resp, err := s.do(ctx, t.URL)
	if err != nil {
		s.log.Warn("target unreachable", "target", t.Name, "url", t.URL, "err", err)
		return st
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		s.log.Warn("target unhealthy", "target", t.Name, "status", resp.StatusCode)
		return st
	}
	st.OK = true
	st.State = "operational"
	return st
}

func (s *Service) probeReachable2xx(ctx context.Context, t Target) Status {
	st := Status{Name: t.Name, State: "unreachable"}
	resp, err := s.do(ctx, t.URL)
	if err != nil {
		s.log.Warn("target unreachable", "target", t.Name, "url", t.URL, "err", err)
		return st
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.log.Warn("target unhealthy", "target", t.Name, "status", resp.StatusCode)
		return st
	}
	st.OK = true
	st.State = "operational"
	return st
}

func (s *Service) probeAPIHealth(ctx context.Context, t Target) Status {
	st := Status{Name: t.Name, State: "unreachable"}
	resp, err := s.do(ctx, t.URL)
	if err != nil {
		s.log.Warn("api health unreachable", "target", t.Name, "url", t.URL, "err", err)
		return st
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.log.Warn("api health bad status", "target", t.Name, "status", resp.StatusCode)
		return st
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		s.log.Warn("api health decode failed", "target", t.Name, "err", err)
		st.State = "error"
		return st
	}

	switch body.Status {
	case "operational":
		st.OK = true
		st.State = "operational"
	case "maintenance", "off":
		s.log.Warn("api not operational", "target", t.Name, "status", body.Status)
		st.State = body.Status
	default:
		s.log.Warn("api health unknown status", "target", t.Name, "status", body.Status)
		st.State = "error"
	}
	return st
}

func (s *Service) do(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return s.http.Do(req)
}
