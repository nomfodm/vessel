// Package system exposes host facts and OS integrations the UI needs: total
// physical RAM (ceiling for the memory slider), opening a profile's folder in the
// native file manager, and relocating the data root to another folder.
package system

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pbnjay/memory"

	"github.com/nomfodm/vessel/internal/paths"
)

// DataRootStore persists the chosen data root (implemented by config.Service).
type DataRootStore interface {
	SetDataRoot(root string) error
}

// FolderPicker shows a native directory chooser. Implemented in main over the
// Wails dialog API so this package stays free of the GUI framework.
type FolderPicker interface {
	Pick(title, startDir string) (string, error)
}

type Service struct {
	layout paths.Layout
	cfg    DataRootStore
	picker FolderPicker
	log    *slog.Logger
}

func New(layout paths.Layout, cfg DataRootStore, picker FolderPicker, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{layout: layout, cfg: cfg, picker: picker, log: log.With("svc", "system")}
}

// TotalRAMMB is the machine's total physical memory in MiB — the upper bound for
// the RAM slider. Returns 0 if the platform can't report it.
func (s *Service) TotalRAMMB() int {
	return int(memory.TotalMemory() / (1024 * 1024))
}

// ProfileDir is the on-disk directory holding a profile's game files.
func (s *Service) ProfileDir(slug string) string {
	return s.layout.ProfileDir(slug)
}

// CurrentDataRoot is where all game data lives right now.
func (s *Service) CurrentDataRoot() string {
	return s.layout.Root
}

// OpenProfileDir opens the profile's folder in the OS file manager, creating it
// first so there's something to show before the first launch.
func (s *Service) OpenProfileDir(slug string) error {
	dir := s.layout.ProfileDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	s.log.Info("opening profile dir", "slug", slug, "dir", dir)
	return openInFileManager(dir)
}

// OpenDataRoot opens the current data root in the OS file manager.
func (s *Service) OpenDataRoot() error {
	if err := os.MkdirAll(s.layout.Root, 0o755); err != nil {
		return err
	}
	s.log.Info("opening data root", "dir", s.layout.Root)
	return openInFileManager(s.layout.Root)
}

// PickDataRoot shows a directory chooser starting at the current root. Returns ""
// when the user cancels.
func (s *Service) PickDataRoot() (string, error) {
	if s.picker == nil {
		return "", errors.New("folder picker unavailable")
	}
	return s.picker.Pick("Папка для файлов Infinity", s.layout.Root)
}

// ChangeDataRoot moves the data tree to newRoot and persists the choice. The
// caller must Restart afterwards: the running services still hold the old layout.
func (s *Service) ChangeDataRoot(newRoot string) error {
	old := filepath.Clean(s.layout.Root)
	newRoot = filepath.Clean(newRoot)

	if err := validateNewRoot(old, newRoot, s.layout.DataDirs()); err != nil {
		return err
	}

	s.log.Info("relocating data root", "from", old, "to", newRoot)
	if err := relocate(old, newRoot, s.layout.DataDirs(), []string{"objects"}, s.log); err != nil {
		return fmt.Errorf("move data: %w", err)
	}

	// Store the default location as "" so it stays the implicit default (keeps
	// following %LOCALAPPDATA% and lets the user "return to default" cleanly).
	stored := newRoot
	if newRoot == filepath.Clean(paths.DefaultDataRoot()) {
		stored = ""
	}
	if err := s.cfg.SetDataRoot(stored); err != nil {
		return fmt.Errorf("persist new root: %w", err)
	}
	s.log.Info("data root relocated; restart required")
	return nil
}

// Restart relaunches the executable and exits — used after ChangeDataRoot so the
// new layout is picked up.
func (s *Service) Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s.log.Info("restarting", "exe", exe)
	if err := exec.Command(exe, os.Args[1:]...).Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil // unreachable
}

// validateNewRoot guards against destructive or pointless moves. The destination
// may hold unrelated files (the default root keeps config.json and logs/); it's
// rejected only if it already contains game data, which a move would collide with.
func validateNewRoot(old, new string, dataDirs []string) error {
	if new == "" {
		return errors.New("пустой путь")
	}
	if new == old {
		return errors.New("это та же папка")
	}
	if isInside(old, new) || isInside(new, old) {
		return errors.New("нельзя выбрать вложенную папку")
	}
	if err := os.MkdirAll(new, 0o755); err != nil {
		return err
	}
	for _, d := range dataDirs {
		if _, err := os.Stat(filepath.Join(new, d)); err == nil {
			return fmt.Errorf("в папке уже есть данные игры (%s) — выберите другую", d)
		}
	}
	return nil
}

// isInside reports whether child is located within parent.
func isInside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// openInFileManager launches the platform's file browser at path. It only starts
// the process (no Wait): explorer.exe in particular returns a non-zero code even
// on success.
func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
