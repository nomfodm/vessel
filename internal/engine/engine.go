package engine

import (
	"log/slog"
	"os"

	"github.com/nomfodm/vessel/internal/paths"
	"golang.org/x/sync/singleflight"
)

type Engine struct {
	layout paths.Layout
	store  *store
	mat    Materializer
	log    *slog.Logger
	sf     singleflight.Group
}

func New(layout paths.Layout, log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}
	log = log.With("svc", "engine")

	if err := os.MkdirAll(layout.ObjectsDir(), 0o755); err != nil {
		return nil, err
	}

	var mat Materializer
	kind := "copy"
	if canHardlink(layout.Root) {
		mat = linkMaterializer{}
		kind = "hardlink"
	} else {
		mat = copyMaterializer{}
	}
	log.Info("file engine ready", "materializer", kind, "root", layout.Root)

	return &Engine{
		layout: layout,
		store:  newStore(layout.ObjectsDir()),
		mat:    mat,
		log:    log,
	}, nil
}

// materialize places a stored object into the game tree at dst.
func (e *Engine) materialize(sha, dst string) error {
	return e.mat.Materialize(e.store.Path(sha), dst)
}
