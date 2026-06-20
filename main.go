package main

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/nomfodm/vessel/internal/auth"
	"github.com/nomfodm/vessel/internal/config"
	"github.com/nomfodm/vessel/internal/engine"
	"github.com/nomfodm/vessel/internal/game"
	"github.com/nomfodm/vessel/internal/health"
	"github.com/nomfodm/vessel/internal/logging"
	"github.com/nomfodm/vessel/internal/paths"
	"github.com/nomfodm/vessel/internal/ping"
	"github.com/nomfodm/vessel/internal/profile"
	"github.com/nomfodm/vessel/internal/system"
	"github.com/nomfodm/vessel/internal/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Defaults point at the public infrastructure; override at build time via
// -ldflags "-X main.<name>=...". These are public URLs, not secrets.
var (
	version        = "dev"
	s3BaseURL      = "https://storage.infinityserver.ru/" // index.json, manifests, files/<ab>/<sha>
	authBaseURL    = "https://api.infinityserver.ru"      // auth API
	authlibBaseURL = "https://api.infinityserver.ru/v1/launcher" // authlib-injector ALI endpoint
)

// wailsEmitter adapts the Wails event bus to game.Emitter. The app pointer is
// set after the application is built (services are constructed before it).
type wailsEmitter struct{ app *application.App }

func (e *wailsEmitter) Emit(event string, data any) {
	if e.app != nil {
		e.app.Event.Emit(event, data)
	}
}

// wailsPicker adapts the Wails directory dialog to system.FolderPicker. Like the
// emitter, its app pointer is set after the application is built.
type wailsPicker struct{ app *application.App }

func (p *wailsPicker) Pick(title, startDir string) (string, error) {
	if p.app == nil {
		return "", errors.New("application not ready")
	}
	d := p.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle(title)
	if startDir != "" {
		d.SetDirectory(startDir)
	}
	return d.PromptForSingleSelection()
}

// tokenKeyMaterial derives a stable per-machine key for the encrypted token
// fallback. Not a secret store — just makes a copied file useless elsewhere.
func tokenKeyMaterial() string {
	host, _ := os.Hostname()
	u, _ := user.Current()
	username := ""
	if u != nil {
		username = u.Username
	}
	return host + "|" + username
}

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	app        *application.App
	mainWindow *application.WebviewWindow
}

func main() {
	// Clean up the .old binary left by a previous self-update (Windows can't
	// delete a running exe, so selfupdate renames it; we remove it on next start).
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}

	dev := version == "dev"

	logger, closer, err := logging.New(paths.LogDir(), dev)
	if err != nil {
		log.Fatalf("init logging: %v", err)
	}
	defer closer.Close()
	slog.SetDefault(logger)

	logger.Info("vessel starting", "version", version, "dev", dev)
	logger.Debug("resolved paths",
		"config", paths.ConfigFile(),
		"dataRoot", paths.DefaultDataRoot(),
		"logs", paths.LogDir(),
		"cache", paths.CacheDir(),
	)

	cfg := config.New(paths.ConfigFile(), logger)
	if err := cfg.Load(); err != nil {
		logger.Error("load config failed, continuing with defaults", "err", err)
	}

	layout := paths.Resolve(cfg.DataRoot())
	logger.Info("data layout resolved", "root", layout.Root)

	eng, err := engine.New(layout, logger)
	if err != nil {
		logger.Error("init file engine failed", "err", err)
		log.Fatalf("init engine: %v", err)
	}

	// One shared transport pools keep-alive connections and reuses TLS sessions
	// across every HTTP service (storage + auth). ResponseHeaderTimeout bounds a
	// dead server without capping large file downloads, so per-client Timeout is
	// only set where the whole exchange is small (auth, health).
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 8
	transport.ResponseHeaderTimeout = 15 * time.Second

	// No total Timeout: profile's client also drives large file downloads.
	profiles := profile.New(s3BaseURL, &http.Client{Transport: transport}, logger)

	authSvc := auth.New(
		auth.NewBackend(authBaseURL, &http.Client{Transport: transport, Timeout: 15 * time.Second}),
		auth.NewStore(filepath.Join(paths.CacheDir(), "token.enc"), tokenKeyMaterial()),
		logger,
	)
	if _, err := authSvc.Restore(context.Background()); err != nil {
		logger.Warn("session restore failed", "err", err)
	}

	emitter := &wailsEmitter{}
	gameSvc := game.New(layout, profiles, eng, authSvc, cfg, emitter, authlibBaseURL, logger)

	pingSvc := ping.New(logger)

	picker := &wailsPicker{}
	systemSvc := system.New(layout, cfg, picker, logger)

	// No total Timeout: Apply downloads the whole new binary. ResponseHeaderTimeout
	// on the shared transport still bounds an unresponsive server.
	updaterSvc := updater.New(version, authBaseURL, &http.Client{Transport: transport}, emitter, logger)

	healthSvc := health.New([]health.Target{
		{Name: "Сервер обновлений", URL: s3BaseURL + "index.json", Kind: health.Reachable},
		{Name: "Сервер авторизации", URL: authBaseURL + "/health", Kind: health.APIHealth},
	}, &http.Client{Transport: transport, Timeout: 8 * time.Second}, logger)

	app := App{}
	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app.app = application.New(application.Options{
		Name:        "Infinity Launcher",
		Description: "Infinity Launcher",
		Services: []application.Service{
			application.NewService(cfg),
			application.NewService(profiles),
			application.NewService(authSvc),
			application.NewService(gameSvc),
			application.NewService(healthSvc),
			application.NewService(pingSvc),
			application.NewService(systemSvc),
			application.NewService(updaterSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.mainWindow = app.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Infinity Launcher",
		Frameless: true,
		Width:     960,
		Height:    600,
		MinWidth:  960,
		MaxWidth:  960,
		MinHeight: 600,
		MaxHeight: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(5, 5, 14),
		URL:              "/",
	})

	// App now exists: wire it into the emitter and picker, which needed the app
	// instance that only exists after application.New.
	emitter.app = app.app
	picker.app = app.app

	logger.Info("running application")
	err = app.app.Run()
	if err != nil {
		logger.Error("application exited with error", "err", err)
		log.Fatal(err)
	}
	logger.Info("vessel stopped")
}
