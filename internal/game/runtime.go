package game

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nomfodm/vessel/internal/engine"
)

// writeMarker atomically writes content to path (temp-file + rename) so a crash
// mid-write never leaves a zero-byte marker that would be mistaken for a valid
// fingerprint, causing an unnecessary full JRE re-sync on the next launch.
func writeMarker(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "runtime-marker-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// syncJob is one manifest to materialize. bytes is its precomputed desired size
// (for unified progress); onDone runs after a successful sync, e.g. to write the
// runtime marker.
type syncJob struct {
	manifest engine.Manifest
	stateKey string
	enabled  []string
	bytes    int64
	onDone   func() error
}

// planSync builds the ordered set of syncs a launch needs: the JRE runtime (only
// when missing or out of date) followed by the profile's files. Byte totals are
// resolved up front so runJobs can render one continuous progress bar.
func (s *Service) planSync(ctx context.Context, slug string, profile engine.Manifest, enabled []string) ([]syncJob, error) {
	var jobs []syncJob

	name := profile.Runtime
	rt, err := s.provider.RuntimeManifest(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("runtime %q: %w", name, err)
	}
	if s.runtimeCurrent(name, rt) {
		s.log.Debug("runtime up to date", "name", name)
	} else {
		s.log.Info("runtime needs provisioning", "name", name, "files", len(rt.Files))
		marker, want := s.runtimeMarker(name), runtimeFingerprint(rt)
		jobs = append(jobs, syncJob{
			manifest: rt,
			stateKey: "runtime-" + name,
			bytes:    engine.DesiredBytes(rt, nil),
			onDone:   func() error { return writeMarker(marker, want) },
		})
	}

	jobs = append(jobs, syncJob{
		manifest: profile,
		stateKey: slug,
		enabled:  enabled,
		bytes:    engine.DesiredBytes(profile, enabled),
	})
	return jobs, nil
}

// runJobs executes the plan, emitting sync:progress as a single bar: each job's
// reported bytes are offset by the total of the jobs before it, against one grand
// total.
func (s *Service) runJobs(ctx context.Context, jobs []syncJob) error {
	var grand int64
	for _, j := range jobs {
		grand += j.bytes
	}

	var base int64
	for _, j := range jobs {
		offset := base
		prog := func(done, _ int64, file string) {
			s.emit.Emit("sync:progress", map[string]any{"done": offset + done, "total": grand, "file": file})
		}
		if err := s.syncer.Sync(ctx, j.manifest, j.stateKey, j.enabled, s.provider.Fetcher(), prog); err != nil {
			return err
		}
		if j.onDone != nil {
			if err := j.onDone(); err != nil {
				return err
			}
		}
		base += j.bytes
	}
	return nil
}

// runtimeCurrent reports whether the runtime marker matches the manifest — i.e.
// the JRE is already provisioned at this exact version, so it can be skipped.
func (s *Service) runtimeCurrent(name string, m engine.Manifest) bool {
	cur, err := os.ReadFile(s.runtimeMarker(name))
	return err == nil && strings.TrimSpace(string(cur)) == runtimeFingerprint(m)
}

// runtimeMarker is a sibling of the runtime dir (not inside it) so the manifest's
// own strict-dir pruning never deletes it.
func (s *Service) runtimeMarker(name string) string {
	return filepath.Join(s.layout.RuntimesDir(), name+".synced")
}

// runtimeFingerprint is a stable hash of the runtime's (path, sha) pairs. It
// changes only when the runtime's contents change, invalidating the marker across
// JRE updates. Relies on the manifest's files being in a stable order, which the
// baker guarantees (sorted by path).
func runtimeFingerprint(m engine.Manifest) string {
	h := sha256.New()
	for _, f := range m.Files {
		io.WriteString(h, f.Path)
		io.WriteString(h, "\x00")
		io.WriteString(h, f.SHA256)
		io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}
