// Package updater reports the launcher's own version, asks the API whether a
// newer release exists, and — unlike a plain version check — can download it,
// verify its SHA256, atomically replace the running binary and relaunch.
package updater

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"
)

// Emitter abstracts Wails event emission (download progress).
type Emitter interface {
	Emit(event string, data any)
}

// Info is the UI-facing result of an update check. When NeedsUpdate is false the
// release fields are empty.
type Info struct {
	Current     string   `json:"current"`
	NeedsUpdate bool     `json:"needsUpdate"`
	Latest      string   `json:"latest"`
	DownloadURL string   `json:"downloadURL"`
	Checksum    string   `json:"checksum"`
	FileSize    int64    `json:"fileSize"`
	Changelog   []string `json:"changelog"`
	IsMandatory bool     `json:"isMandatory"`
}

type Service struct {
	current string
	baseURL string
	http    *http.Client
	emit    Emitter
	log     *slog.Logger
}

func New(current, apiBaseURL string, client *http.Client, emit Emitter, log *slog.Logger) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		current: current,
		baseURL: strings.TrimRight(apiBaseURL, "/"),
		http:    client,
		emit:    emit,
		log:     log.With("svc", "updater"),
	}
}

func (s *Service) Version() string {
	return s.current
}

// Check asks GET /v1/launcher/update?platform=&version= whether a newer release
// is available. The server decides needs_update; we just relay it.
func (s *Service) Check(ctx context.Context) (Info, error) {
	// The dev build has no real version — don't send garbage to the SemVer API.
	if s.current == "dev" {
		return Info{Current: s.current}, nil
	}

	u := fmt.Sprintf("%s/v1/launcher/update?platform=%s&version=%s",
		s.baseURL, platform(), url.QueryEscape(s.current))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Info{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.log.Warn("update check failed", "err", err)
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("GET /v1/launcher/update: status %d", resp.StatusCode)
	}

	var body struct {
		NeedsUpdate bool `json:"needs_update"`
		Latest      *struct {
			Version     string   `json:"version"`
			DownloadURL string   `json:"download_url"`
			Checksum    string   `json:"checksum"`
			FileSize    int64    `json:"file_size"`
			Changelog   []string `json:"changelog"`
			IsMandatory bool     `json:"is_mandatory"`
		} `json:"latest"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Info{}, err
	}

	info := Info{Current: s.current, NeedsUpdate: body.NeedsUpdate}
	if body.Latest != nil {
		info.Latest = body.Latest.Version
		info.DownloadURL = body.Latest.DownloadURL
		info.Checksum = body.Latest.Checksum
		info.FileSize = body.Latest.FileSize
		info.Changelog = body.Latest.Changelog
		info.IsMandatory = body.Latest.IsMandatory
	}
	s.log.Info("update check", "current", s.current, "latest", info.Latest, "needsUpdate", info.NeedsUpdate)
	return info, nil
}

// Apply re-checks for an update, downloads it (emitting update:progress), verifies
// its SHA256 and atomically replaces the running binary. It does NOT restart —
// call Restart after a successful Apply. The check is re-run here so the download
// URL and checksum are server-authoritative, not passed from the UI.
func (s *Service) Apply(ctx context.Context) error {
	info, err := s.Check(ctx)
	if err != nil {
		return err
	}
	if !info.NeedsUpdate || info.DownloadURL == "" {
		return errors.New("no update available")
	}
	checksum, err := hex.DecodeString(strings.TrimSpace(info.Checksum))
	if err != nil || len(checksum) == 0 {
		return fmt.Errorf("release has no valid checksum, refusing to update")
	}

	body, err := s.download(ctx, info.DownloadURL, info.FileSize)
	if err != nil {
		return err
	}
	defer body.Close()

	// selfupdate verifies the SHA256 against checksum before swapping, then does
	// the platform-safe rename dance (works even while the .exe is running).
	if err := selfupdate.Apply(body, selfupdate.Options{Checksum: checksum}); err != nil {
		if rb := selfupdate.RollbackError(err); rb != nil {
			s.log.Error("update failed AND rollback failed — binary may be broken", "err", rb)
		}
		s.log.Error("apply update failed", "err", err)
		return err
	}
	s.log.Info("update applied", "version", info.Latest)
	return nil
}

// Restart launches the (now replaced) executable and exits the current process.
func (s *Service) Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s.log.Info("restarting into updated binary", "exe", exe)
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil // unreachable
}

// download streams the URL into memory, emitting update:progress as it goes.
func (s *Service) download(ctx context.Context, dlURL string, total int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: status %d", dlURL, resp.StatusCode)
	}
	if total == 0 {
		total = resp.ContentLength
	}
	return &progressReader{rc: resp.Body, total: total, emit: s.emit}, nil
}

type progressReader struct {
	rc    io.ReadCloser
	total int64
	read  int64
	emit  Emitter
	lastP int
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.rc.Read(b)
	p.read += int64(n)
	if p.emit != nil && p.total > 0 {
		if pct := int(p.read * 100 / p.total); pct != p.lastP {
			p.lastP = pct
			p.emit.Emit("update:progress", map[string]any{"done": p.read, "total": p.total, "pct": pct})
		}
	}
	return n, err
}

func (p *progressReader) Close() error { return p.rc.Close() }

// platform maps the build OS to the API's Platform value.
func platform() string {
	switch runtime.GOOS {
	case "windows":
		return "win"
	case "darwin":
		return "mac"
	default:
		return "linux"
	}
}
