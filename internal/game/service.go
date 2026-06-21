package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	RuntimeManifest(ctx context.Context, name string) (engine.Manifest, error)
	Fetcher() engine.Fetcher
}

type Syncer interface {
	Sync(ctx context.Context, m engine.Manifest, stateKey string, enabledOptional []string, fetch engine.Fetcher, onProgress engine.ProgressFunc) error
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
	layout      paths.Layout
	provider    Provider
	syncer      Syncer
	session     Sessioner
	settings    Settings
	emit        Emitter
	authlibURL  string
	run         runner
	env         Env
	log         *slog.Logger

	mu      sync.Mutex
	running *process
	// cancel aborts the in-flight launch (sync/session/prepare). Non-nil from the
	// start of Launch until the process exits or the launch fails — it also guards
	// against a second concurrent Launch.
	cancel context.CancelFunc
}

func New(layout paths.Layout, provider Provider, syncer Syncer, session Sessioner, settings Settings, emit Emitter, authlibURL string, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		layout:     layout,
		provider:   provider,
		syncer:     syncer,
		session:    session,
		settings:   settings,
		emit:       emit,
		authlibURL: authlibURL,
		run:        osRunner{},
		env:        CurrentEnv(),
		log:        log.With("svc", "game"),
	}
}

// Launch runs the full chain: sync files -> issue session -> build command ->
// start the JVM and stream its lifecycle as events.
func (s *Service) Launch(ctx context.Context, slug string) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	// Until the process is handed to pump(), any early return must tear the launch
	// down here (release the context, clear the guard). Once handed off, pump()
	// owns cleanup on exit.
	handed := false
	defer func() {
		if !handed {
			cancel()
			s.mu.Lock()
			s.cancel = nil
			s.mu.Unlock()
		}
	}()

	cfg := s.settings.Snapshot()
	pc := cfg.Profiles[slug]

	manifest, err := s.provider.Manifest(ctx, slug)
	if err != nil {
		return err
	}
	if manifest.Runtime == "" {
		return fmt.Errorf("profile %q has no runtime", slug)
	}

	// Plan the JRE runtime and the profile files as one set of jobs so progress is
	// a single bar across everything that actually needs downloading this launch.
	jobs, err := s.planSync(ctx, slug, manifest, pc.EnabledOptional)
	if err != nil {
		return err
	}
	s.log.Info("syncing", "slug", slug, "jobs", len(jobs))
	if err := s.runJobs(ctx, jobs); err != nil {
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
	s.log.Debug("launch argv", "argv", argv)

	proc, err := s.run.Start(params.JavaPath, argv[1:], params.GameDir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.running = proc
	s.mu.Unlock()
	handed = true
	s.emit.Emit("game:started", map[string]any{"slug": slug})

	go s.pump(slug, proc)
	return nil
}

// pump streams the process output as game:log and emits game:exited on exit.
func (s *Service) pump(slug string, proc *process) {
	for line := range proc.Lines() {
		s.log.Debug("game output", "slug", slug, "line", line)
		s.emit.Emit("game:log", map[string]any{"line": line})
	}
	exitCode := proc.Wait()

	s.mu.Lock()
	s.running = nil
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()

	s.log.Info("game exited", "slug", slug, "code", exitCode)
	s.emit.Emit("game:exited", map[string]any{"slug": slug, "code": exitCode})
}

// Stop aborts whatever the service is doing: it cancels an in-flight launch
// (sync/prepare) and kills the game process if one is running.
func (s *Service) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	proc := s.running
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
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
		AssetsDir:   filepath.Join(profileDir, "assets"),
		LibsDir:     filepath.Join(profileDir, "libraries"),
		ClientJar:   filepath.Join(profileDir, "client.jar"),
		JavaPath:    javaExe(runtimeDir, s.env),
		NativesDir:  filepath.Join(profileDir, "natives"),
		VersionName: slug,
		PlayerName:  sess.Username,
		UUID:        sess.UUID,
		AccessToken: sess.AccessToken,
		MaxRAMMB:    ramMB(pc.RAMMB),
		ExtraJVMArg: s.authlibArgs(profileDir, pc.ExtraJVMArgs),
	}
}

// authlibArgs prepends the -javaagent authlib-injector flag when the jar is
// present in the profile directory and authlibURL is configured.
func (s *Service) authlibArgs(profileDir string, extra []string) []string {
	if s.authlibURL == "" {
		return extra
	}
	jar := filepath.Join(profileDir, "auth.jar")
	if _, err := os.Stat(jar); err != nil {
		return extra
	}
	agent := "-javaagent:" + jar + "=" + s.authlibURL
	return append([]string{agent}, extra...)
}

// ramMB converts the stored config value to a -Xmx argument: values ≤ 1024 mean
// AUTO (no -Xmx flag), matching the UI slider's leftmost "АВТО" label at 1024.
func ramMB(configured int) int {
	if configured <= 1024 {
		return 0
	}
	return configured
}

func javaExe(runtimeDir string, env Env) string {
	bin := "java"
	if env.OS == "windows" {
		bin = "javaw.exe"
	}
	return filepath.Join(runtimeDir, "bin", bin)
}
