package main

import (
	"context"
	"embed"
	_ "embed"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nomfodm/vessel/internal/auth"
	"github.com/nomfodm/vessel/internal/config"
	"github.com/nomfodm/vessel/internal/engine"
	"github.com/nomfodm/vessel/internal/game"
	"github.com/nomfodm/vessel/internal/logging"
	"github.com/nomfodm/vessel/internal/paths"
	"github.com/nomfodm/vessel/internal/profile"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Defaults point at the public infrastructure; override at build time via
// -ldflags "-X main.<name>=...". These are public URLs, not secrets.
var (
	version     = "dev"
	s3BaseURL   = "https://storage.infinityserver.ru/" // index.json, manifests, files/<ab>/<sha>
	authBaseURL = "https://api.infinityserver.ru"      // auth API
)

// wailsEmitter adapts the Wails event bus to game.Emitter. The app pointer is
// set after the application is built (services are constructed before it).
type wailsEmitter struct{ app *application.App }

func (e *wailsEmitter) Emit(event string, data any) {
	if e.app != nil {
		e.app.Event.Emit(event, data)
	}
}

// tokenKeyMaterial derives a stable per-machine key for the encrypted token
// fallback. Not a secret store — just makes a copied file useless elsewhere.
func tokenKeyMaterial() string {
	host, _ := os.Hostname()
	return host + "|" + os.Getenv("USERNAME") + os.Getenv("USER")
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

	profiles := profile.New(s3BaseURL, nil, logger)

	authSvc := auth.New(
		auth.NewBackend(authBaseURL, nil),
		auth.NewStore(filepath.Join(paths.CacheDir(), "token.enc"), tokenKeyMaterial()),
		logger,
	)
	if _, err := authSvc.Restore(context.Background()); err != nil {
		logger.Warn("session restore failed", "err", err)
	}

	emitter := &wailsEmitter{}
	gameSvc := game.New(layout, profiles, eng, authSvc, cfg, emitter, logger)

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

	// App now exists: wire it into the emitter so game events reach the frontend.
	emitter.app = app.app

	logger.Info("running application")
	err = app.app.Run()
	if err != nil {
		logger.Error("application exited with error", "err", err)
		log.Fatal(err)
	}
	logger.Info("vessel stopped")
}
