package logging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lmittmann/tint"
)

const (
	filePrefix = "launcher-"
	keepFiles  = 5
)

func New(dir string, dev bool) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}

	name := filePrefix + time.Now().Format("20060102-150405") + ".log"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	handlers := []slog.Handler{
		tint.NewHandler(f, &tint.Options{Level: slog.LevelDebug, NoColor: true}),
	}
	if dev {
		handlers = append(handlers, tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}))
	}

	logger := slog.New(fanout{handlers})

	if err := prune(dir, keepFiles); err != nil {
		logger.Warn("prune old logs", "err", err)
	}

	return logger, f, nil
}

// prune deletes oldest log files, keeping the most recent keep. The timestamp in
// the filename sorts lexically == chronologically, so plain string sort works.
func prune(dir string, keep int) error {
	files, err := filepath.Glob(filepath.Join(dir, filePrefix+"*.log"))
	if err != nil {
		return err
	}
	if len(files) <= keep {
		return nil
	}
	sort.Strings(files)

	var errs []error
	for _, old := range files[:len(files)-keep] {
		if err := os.Remove(old); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// fanout dispatches each record to every underlying handler, letting console and
// file use different formats.
type fanout struct {
	handlers []slog.Handler
}

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return fanout{hs}
}

func (f fanout) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithGroup(name)
	}
	return fanout{hs}
}
