package game

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/nomfodm/vessel/internal/auth"
	"github.com/nomfodm/vessel/internal/config"
	"github.com/nomfodm/vessel/internal/engine"
	"github.com/nomfodm/vessel/internal/paths"
)

var ErrAlreadyRunning = errors.New("game already running")

// Collaborators, as narrow interfaces so the service is testable in isolation.

type Provider interface {
	Manifest(ctx context.Context, slug string) (engine.Manifest, error)
	Fetcher() engine.Fetcher
}

type Syncer interface {
	Sync(ctx context.Context, m engine.Manifest, enabledOptional []string, fetch engine.Fetcher, onProgress engine.ProgressFunc) error
}

type Sessioner interface {
	GameSession(ctx context.Context) (auth.GameSession, error)
}

type Settings interface {
	Snapshot() config.Config
}

// Emitter abstracts Wails event emission.
type Emitter interface {
	Emit(event string, data any)
}

type Service struct {
	layout   paths.Layout
	provider Provider
	syncer   Syncer
	session  Sessioner
	settings Settings
	emit     Emitter
	run      runner
	env      Env
	log      *slog.Logger

	mu      sync.Mutex
	running *process
}

func New(layout paths.Layout, provider Provider, syncer Syncer, session Sessioner, settings Settings, emit Emitter, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		layout:   layout,
		provider: provider,
		syncer:   syncer,
		session:  session,
		settings: settings,
		emit:     emit,
		run:      osRunner{},
		env:      CurrentEnv(),
		log:      log.With("svc", "game"),
	}
}

// Launch runs the full chain: sync files -> issue session -> build command ->
// start the JVM and stream its lifecycle as events.
func (s *Service) Launch(ctx context.Context, slug string) error {
	s.mu.Lock()
	if s.running != nil {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.mu.Unlock()

	cfg := s.settings.Snapshot()
	pc := cfg.Profiles[slug]

	manifest, err := s.provider.Manifest(ctx, slug)
	if err != nil {
		return err
	}

	s.log.Info("syncing files", "slug", slug)
	onProgress := func(done, total int, file string) {
		s.emit.Emit("sync:progress", map[string]any{"done": done, "total": total, "file": file})
	}
	if err := s.syncer.Sync(ctx, manifest, pc.EnabledOptional, s.provider.Fetcher(), onProgress); err != nil {
		return err
	}

	sess, err := s.session.GameSession(ctx)
	if err != nil {
		return err
	}

	version, err := ParseVersionFile(filepath.Join(s.layout.ProfileDir(slug), "version.json"))
	if err != nil {
		return err
	}

	params := s.launchParams(slug, manifest, pc, sess)
	argv := BuildCommand(version, params, s.env)
	s.log.Info("launching", "slug", slug, "java", params.JavaPath, "args", len(argv))

	proc, err := s.run.Start(params.JavaPath, argv[1:], params.GameDir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.running = proc
	s.mu.Unlock()
	s.emit.Emit("game:started", map[string]any{"slug": slug})

	go s.pump(slug, proc)
	return nil
}

// pump streams the process output as game:log and emits game:exited on exit.
func (s *Service) pump(slug string, proc *process) {
	for line := range proc.Lines() {
		s.emit.Emit("game:log", map[string]any{"line": line})
	}
	exitCode := proc.Wait()

	s.mu.Lock()
	s.running = nil
	s.mu.Unlock()

	s.log.Info("game exited", "slug", slug, "code", exitCode)
	s.emit.Emit("game:exited", map[string]any{"slug": slug, "code": exitCode})
}

func (s *Service) Stop() error {
	s.mu.Lock()
	proc := s.running
	s.mu.Unlock()
	if proc == nil {
		return nil
	}
	s.log.Info("killing game process")
	return proc.Kill()
}

func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running != nil
}

func (s *Service) launchParams(slug string, m engine.Manifest, pc config.ProfileConfig, sess auth.GameSession) LaunchParams {
	profileDir := s.layout.ProfileDir(slug)
	runtimeDir := filepath.Join(s.layout.RuntimesDir(), m.Runtime)

	return LaunchParams{
		GameDir:     profileDir,
		AssetsDir:   filepath.Join(s.layout.Root, "assets"),
		LibsDir:     filepath.Join(s.layout.Root, "libraries"),
		ClientJar:   filepath.Join(profileDir, "client.jar"),
		JavaPath:    javaExe(runtimeDir, s.env),
		NativesDir:  filepath.Join(profileDir, "natives"),
		VersionName: slug,
		PlayerName:  sess.Username,
		UUID:        sess.UUID,
		AccessToken: sess.AccessToken,
		MaxRAMMB:    pc.RAMMB,
		ExtraJVMArg: pc.ExtraJVMArgs,
	}
}

func javaExe(runtimeDir string, env Env) string {
	bin := "java"
	if env.OS == "windows" {
		bin = "javaw.exe"
	}
	return filepath.Join(runtimeDir, "bin", bin)
}
