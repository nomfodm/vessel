package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/nomfodm/vessel/internal/engine"
)

type Server struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Profile struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	MCVersion   string `json:"mcVersion"`
	Description string `json:"desc"`

	// Presentation fields, authored in index.json on S3 (not computed):
	Icon      string `json:"icon"`
	Accent    string `json:"accent"`
	AccentDim string `json:"accentDim"`
	Bg        string `json:"bg"`

	Order        int      `json:"order"`
	Hidden       bool     `json:"hidden"`
	Servers      []Server `json:"servers"`
	ManifestPath string   `json:"manifestPath"`
}

type index struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

type Service struct {
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

func New(baseURL string, client *http.Client, log *slog.Logger) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		baseURL: baseURL,
		http:    client,
		log:     log.With("svc", "profile"),
	}
}

// Profiles returns visible profiles sorted by Order — for the UI list.
func (s *Service) Profiles(ctx context.Context) ([]Profile, error) {
	idx, err := s.fetchIndex(ctx)
	if err != nil {
		return nil, err
	}

	visible := make([]Profile, 0, len(idx.Profiles))
	for _, p := range idx.Profiles {
		if !p.Hidden {
			visible = append(visible, p)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return visible[i].Order < visible[j].Order
	})

	s.log.Info("profiles loaded", "total", len(idx.Profiles), "visible", len(visible), "indexVersion", idx.Version)
	return visible, nil
}

func (s *Service) fetchIndex(ctx context.Context) (index, error) {
	var idx index
	if err := s.getJSON(ctx, "index.json", &idx); err != nil {
		return index{}, err
	}
	return idx, nil
}

// Manifest searches the full index (including hidden profiles) so a hidden but
// still-launchable profile resolves.
func (s *Service) Manifest(ctx context.Context, slug string) (engine.Manifest, error) {
	idx, err := s.fetchIndex(ctx)
	if err != nil {
		return engine.Manifest{}, err
	}
	var manifestPath string
	for _, p := range idx.Profiles {
		if p.Slug == slug {
			manifestPath = p.ManifestPath
			break
		}
	}
	if manifestPath == "" {
		return engine.Manifest{}, fmt.Errorf("profile %q not found", slug)
	}

	var m engine.Manifest
	if err := s.getJSON(ctx, manifestPath, &m); err != nil {
		return engine.Manifest{}, err
	}
	s.log.Info("manifest loaded", "slug", slug, "files", len(m.Files))
	return m, nil
}

// OptionalFile is the UI-facing view of an optional component: presentation
// metadata from the manifest plus the total size of its files.
type OptionalFile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	SizeBytes int64  `json:"sizeBytes"`
	DefaultOn bool   `json:"defaultOn"`
}

// OptionalFiles returns the optional components of a profile for the toggles UI,
// summing each id's file sizes.
func (s *Service) OptionalFiles(ctx context.Context, slug string) ([]OptionalFile, error) {
	m, err := s.Manifest(ctx, slug)
	if err != nil {
		return nil, err
	}
	sizes := make(map[string]int64)
	for _, f := range m.Files {
		if f.Optional && f.ID != "" {
			sizes[f.ID] += f.Size
		}
	}
	out := make([]OptionalFile, 0, len(m.Optional))
	for _, g := range m.Optional {
		out = append(out, OptionalFile{
			ID:        g.ID,
			Name:      g.Name,
			Desc:      g.Desc,
			SizeBytes: sizes[g.ID],
			DefaultOn: g.DefaultOn,
		})
	}
	return out, nil
}

func (s *Service) Fetcher() engine.Fetcher {
	return &httpFetcher{baseURL: s.baseURL, http: s.http}
}

func (s *Service) getJSON(ctx context.Context, path string, dst any) error {
	u, err := joinURL(s.baseURL, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.log.Error("fetch failed", "url", u, "err", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

type httpFetcher struct {
	baseURL string
	http    *http.Client
}

func (f *httpFetcher) Fetch(ctx context.Context, sha string) (io.ReadCloser, error) {
	u, err := joinURL(f.baseURL, "files/"+sha[:2]+"/"+sha)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("fetch %s: status %d", u, resp.StatusCode)
	}
	return resp.Body, nil
}

func joinURL(base, ref string) (string, error) {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}
