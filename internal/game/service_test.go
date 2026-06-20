package game

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nomfodm/vessel/internal/auth"
	"github.com/nomfodm/vessel/internal/config"
	"github.com/nomfodm/vessel/internal/engine"
	"github.com/nomfodm/vessel/internal/paths"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeProvider struct {
	manifest   engine.Manifest
	err        error
	runtime    engine.Manifest
	runtimeErr error
}

func (f *fakeProvider) Manifest(context.Context, string) (engine.Manifest, error) {
	return f.manifest, f.err
}
func (f *fakeProvider) RuntimeManifest(context.Context, string) (engine.Manifest, error) {
	return f.runtime, f.runtimeErr
}
func (f *fakeProvider) Fetcher() engine.Fetcher { return nil }

// fullSyncer reports a single 100%-of-its-job progress tick, then succeeds.
type fullSyncer struct{}

func (fullSyncer) Sync(_ context.Context, m engine.Manifest, _ string, enabled []string, _ engine.Fetcher, prog engine.ProgressFunc) error {
	if prog != nil {
		total := engine.DesiredBytes(m, enabled)
		prog(total, total, "done")
	}
	return nil
}

// recordingEmitter captures the data of every sync:progress event.
type recordingEmitter struct {
	mu       sync.Mutex
	progress []map[string]any
}

func (e *recordingEmitter) Emit(event string, data any) {
	if event != "sync:progress" {
		return
	}
	e.mu.Lock()
	if m, ok := data.(map[string]any); ok {
		e.progress = append(e.progress, m)
	}
	e.mu.Unlock()
}

type fakeSyncer struct {
	err    error
	called bool
}

func (f *fakeSyncer) Sync(_ context.Context, _ engine.Manifest, _ string, _ []string, _ engine.Fetcher, onProgress engine.ProgressFunc) error {
	f.called = true
	if onProgress != nil {
		onProgress(1, 1, "file.jar")
	}
	return f.err
}

type fakeSessioner struct {
	sess auth.GameSession
	err  error
}

func (f *fakeSessioner) GameSession(context.Context) (auth.GameSession, error) {
	return f.sess, f.err
}

type fakeSettings struct{ cfg config.Config }

func (f *fakeSettings) Snapshot() config.Config { return f.cfg }

type capturingEmitter struct {
	mu     sync.Mutex
	events []string
}

func (e *capturingEmitter) Emit(event string, _ any) {
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
}
func (e *capturingEmitter) has(event string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.events {
		if ev == event {
			return true
		}
	}
	return false
}

// fakeRunner returns a process backed by an in-memory line channel.
type fakeRunner struct {
	startErr  error
	lines     []string
	exitCode  int
	startCald bool
}

func (r *fakeRunner) Start(_ string, _ []string, _ string) (*process, error) {
	r.startCald = true
	if r.startErr != nil {
		return nil, r.startErr
	}
	p := &process{lines: make(chan string, len(r.lines)), waitErr: make(chan error, 1)}
	for _, l := range r.lines {
		p.lines <- l
	}
	close(p.lines)
	p.waitErr <- nil
	return p, nil
}

func newService(t *testing.T, prov Provider, sync Syncer, sess Sessioner, set Settings, emit Emitter, run runner) (*Service, paths.Layout) {
	t.Helper()
	layout := paths.Resolve(t.TempDir())
	s := New(layout, prov, sync, sess, set, emit, "", testLogger())
	s.run = run
	s.env = Env{OS: "linux", Arch: "x64"}
	return s, layout
}

func writeVersionJSON(t *testing.T, layout paths.Layout, slug string) {
	t.Helper()
	dir := layout.ProfileDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"1.20.1","type":"release","mainClass":"net.minecraft.client.main.Main","assetIndex":{"id":"5"},"minecraftArguments":"--username ${auth_player_name}"}`
	if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestLaunchFullChain(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre-21"}}
	sync := &fakeSyncer{}
	sess := &fakeSessioner{sess: auth.GameSession{UUID: "u", Username: "alex", AccessToken: "tok"}}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{"survival": {RAMMB: 2048}}}}
	emit := &capturingEmitter{}
	run := &fakeRunner{lines: []string{"hello from mc", "another line"}}

	s, layout := newService(t, prov, sync, sess, set, emit, run)
	writeVersionJSON(t, layout, "survival")

	if err := s.Launch(context.Background(), "survival"); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	if !sync.called {
		t.Error("sync not called")
	}
	if !run.startCald {
		t.Error("runner.Start not called")
	}

	waitFor(t, func() bool { return emit.has("game:exited") })
	for _, ev := range []string{"sync:progress", "game:started", "game:log", "game:exited"} {
		if !emit.has(ev) {
			t.Errorf("missing event %q", ev)
		}
	}
	if s.IsRunning() {
		t.Error("still running after exit")
	}
}

func TestLaunchRejectsSecond(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre-21"}}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{}}}
	emit := &capturingEmitter{}
	// runner whose process never exits, so the first launch stays "running".
	run := &blockingRunner{released: make(chan struct{})}

	s, layout := newService(t, prov, &fakeSyncer{}, &fakeSessioner{}, set, emit, run)
	writeVersionJSON(t, layout, "survival")

	if err := s.Launch(context.Background(), "survival"); err != nil {
		t.Fatalf("first Launch: %v", err)
	}
	waitFor(t, func() bool { return s.IsRunning() })

	if err := s.Launch(context.Background(), "survival"); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Launch = %v, want ErrAlreadyRunning", err)
	}
	close(run.released)
}

func TestLaunchSyncError(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre-21"}}
	sync := &fakeSyncer{err: errors.New("network down")}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{}}}
	run := &fakeRunner{}

	s, layout := newService(t, prov, sync, &fakeSessioner{}, set, &capturingEmitter{}, run)
	writeVersionJSON(t, layout, "survival")

	if err := s.Launch(context.Background(), "survival"); err == nil {
		t.Fatal("expected sync error")
	}
	if run.startCald {
		t.Error("process started despite sync failure")
	}
}

func TestLaunchSessionError(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre-21"}}
	sess := &fakeSessioner{err: errors.New("not logged in")}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{}}}
	run := &fakeRunner{}

	s, layout := newService(t, prov, &fakeSyncer{}, sess, set, &capturingEmitter{}, run)
	writeVersionJSON(t, layout, "survival")

	if err := s.Launch(context.Background(), "survival"); err == nil {
		t.Fatal("expected session error")
	}
	if run.startCald {
		t.Error("process started despite session failure")
	}
}

func TestPlanSyncSkipsCurrentRuntime(t *testing.T) {
	rt := engine.Manifest{Files: []engine.ManifestFile{{Path: "runtimes/jre/bin/java", SHA256: "abc", Size: 5}}}
	prov := &fakeProvider{runtime: rt}
	s, _ := newService(t, prov, &fakeSyncer{}, &fakeSessioner{}, &fakeSettings{}, &capturingEmitter{}, &fakeRunner{})

	// Mark the runtime as already provisioned at this fingerprint.
	marker := s.runtimeMarker("jre")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte(runtimeFingerprint(rt)), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := s.planSync(context.Background(), "test-profile", engine.Manifest{Runtime: "jre"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1 (current runtime skipped, profile only)", len(jobs))
	}
}

func TestPlanSyncIncludesStaleRuntime(t *testing.T) {
	rt := engine.Manifest{Files: []engine.ManifestFile{{Path: "a", SHA256: "1", Size: 5}}}
	prov := &fakeProvider{runtime: rt}
	s, _ := newService(t, prov, &fakeSyncer{}, &fakeSessioner{}, &fakeSettings{}, &capturingEmitter{}, &fakeRunner{})

	// No marker -> runtime is stale -> it becomes the first job.
	jobs, err := s.planSync(context.Background(), "test-profile", engine.Manifest{Runtime: "jre"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2 (stale runtime + profile)", len(jobs))
	}
	if jobs[0].bytes != 5 {
		t.Fatalf("runtime job bytes = %d, want 5", jobs[0].bytes)
	}
}

func TestRunJobsUnifiedProgress(t *testing.T) {
	emit := &recordingEmitter{}
	s, _ := newService(t, &fakeProvider{}, fullSyncer{}, &fakeSessioner{}, &fakeSettings{}, emit, &fakeRunner{})

	jobs := []syncJob{
		{manifest: engine.Manifest{Files: []engine.ManifestFile{{Path: "r", SHA256: "1", Size: 30}}}, bytes: 30},
		{manifest: engine.Manifest{Files: []engine.ManifestFile{{Path: "p", SHA256: "2", Size: 70}}}, bytes: 70},
	}
	if err := s.runJobs(context.Background(), jobs); err != nil {
		t.Fatal(err)
	}

	// One continuous bar: both emits share grand total 100; the first job's done
	// is offset 0+30, the second's is 30+70.
	if len(emit.progress) != 2 {
		t.Fatalf("progress emits = %d, want 2", len(emit.progress))
	}
	if d, tot := emit.progress[0]["done"].(int64), emit.progress[0]["total"].(int64); d != 30 || tot != 100 {
		t.Fatalf("first progress = %d/%d, want 30/100", d, tot)
	}
	if d, tot := emit.progress[1]["done"].(int64), emit.progress[1]["total"].(int64); d != 100 || tot != 100 {
		t.Fatalf("final progress = %d/%d, want 100/100", d, tot)
	}
}

// blockingSyncer signals when Sync is entered, then blocks until the context is
// cancelled — modelling a long download that Stop must be able to abort.
type blockingSyncer struct{ started chan struct{} }

func (b *blockingSyncer) Sync(ctx context.Context, _ engine.Manifest, _ string, _ []string, _ engine.Fetcher, _ engine.ProgressFunc) error {
	close(b.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestStopCancelsInFlightSync(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre"}}
	sync := &blockingSyncer{started: make(chan struct{})}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{}}}
	run := &fakeRunner{}

	s, layout := newService(t, prov, sync, &fakeSessioner{}, set, &capturingEmitter{}, run)
	writeVersionJSON(t, layout, "survival")

	errc := make(chan error, 1)
	go func() { errc <- s.Launch(context.Background(), "survival") }()

	<-sync.started // the (runtime) sync is now in-flight
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("Launch returned nil, want cancellation error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Launch did not return after Stop")
	}
	if run.startCald {
		t.Error("process started despite cancellation")
	}
	if s.IsRunning() {
		t.Error("still running after cancel")
	}
}

func TestLaunchRuntimeError(t *testing.T) {
	prov := &fakeProvider{manifest: engine.Manifest{Runtime: "jre-21"}, runtimeErr: errors.New("no runtime build")}
	set := &fakeSettings{cfg: config.Config{Profiles: map[string]config.ProfileConfig{}}}
	run := &fakeRunner{}

	s, layout := newService(t, prov, &fakeSyncer{}, &fakeSessioner{}, set, &capturingEmitter{}, run)
	writeVersionJSON(t, layout, "survival")

	if err := s.Launch(context.Background(), "survival"); err == nil {
		t.Fatal("expected runtime provisioning error")
	}
	if run.startCald {
		t.Error("process started despite runtime failure")
	}
}

// blockingRunner yields a process that stays alive until released is closed.
type blockingRunner struct{ released chan struct{} }

func (r *blockingRunner) Start(_ string, _ []string, _ string) (*process, error) {
	p := &process{lines: make(chan string), waitErr: make(chan error, 1)}
	go func() {
		<-r.released
		close(p.lines)
		p.waitErr <- nil
	}()
	return p, nil
}
