package main

import (
	"embed"
	_ "embed"
	"log"
	"log/slog"
	"time"

	"github.com/nomfodm/vessel/internal/config"
	"github.com/nomfodm/vessel/internal/logging"
	"github.com/nomfodm/vessel/internal/paths"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

type App struct {
	app        *application.App
	mainWindow *application.WebviewWindow
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
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
			application.NewService(&GreetService{}),
			application.NewService(cfg),
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

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	logger.Info("running application")
	err = app.app.Run()
	if err != nil {
		logger.Error("application exited with error", "err", err)
		log.Fatal(err)
	}
	logger.Info("vessel stopped")
}
