package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const schemaVersion = 1

type ProfileConfig struct {
	RAMMB           int      `json:"ramMB"`
	EnabledOptional []string `json:"enabledOptional"`
	ExtraJVMArgs    []string `json:"extraJvmArgs"`
}

type Config struct {
	Version     int                      `json:"version"`
	DataRoot    string                   `json:"dataRoot"`
	LastProfile string                   `json:"lastProfile"`
	Profiles    map[string]ProfileConfig `json:"profiles"`
}

type ConfigService struct {
	mu   sync.Mutex
	path string
	cfg  Config
	log  *slog.Logger
}

func New(path string, log *slog.Logger) *ConfigService {
	if log == nil {
		log = slog.Default()
	}
	return &ConfigService{
		path: path,
		cfg:  defaultConfig(),
		log:  log.With("svc", "config"),
	}
}

func defaultConfig() Config {
	return Config{
		Version:  schemaVersion,
		Profiles: map[string]ProfileConfig{},
	}
}

func (s *ConfigService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Debug("loading config", "path", s.path)

	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.log.Info("no config file, writing defaults", "path", s.path)
		s.cfg = defaultConfig()
		return s.saveLocked()
	}
	if err != nil {
		s.log.Error("read config", "path", s.path, "err", err)
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		s.log.Error("parse config", "path", s.path, "err", err)
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	s.cfg = cfg
	s.log.Info("config loaded",
		"path", s.path,
		"version", cfg.Version,
		"dataRoot", cfg.DataRoot,
		"lastProfile", cfg.LastProfile,
		"profiles", len(cfg.Profiles),
	)
	return nil
}

func (s *ConfigService) Snapshot() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Debug("snapshot requested")
	return s.cfg.clone()
}

func (s *ConfigService) DataRoot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.DataRoot
}

func (s *ConfigService) SetDataRoot(root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Info("set dataRoot", "old", s.cfg.DataRoot, "new", root)
	s.cfg.DataRoot = root
	return s.saveLocked()
}

func (s *ConfigService) SetLastProfile(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Info("set lastProfile", "slug", slug)
	s.cfg.LastProfile = slug
	return s.saveLocked()
}

func (s *ConfigService) SetProfileRAM(slug string, ramMB int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Info("set profile RAM", "slug", slug, "ramMB", ramMB)
	p := s.cfg.Profiles[slug]
	p.RAMMB = ramMB
	s.cfg.Profiles[slug] = p
	return s.saveLocked()
}

func (s *ConfigService) SetEnabledOptional(slug string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Info("set enabled optional", "slug", slug, "ids", ids)
	p := s.cfg.Profiles[slug]
	p.EnabledOptional = ids
	s.cfg.Profiles[slug] = p
	return s.saveLocked()
}

func (s *ConfigService) SetExtraJVMArgs(slug string, args []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Info("set extra JVM args", "slug", slug, "args", args)
	p := s.cfg.Profiles[slug]
	p.ExtraJVMArgs = args
	s.cfg.Profiles[slug] = p
	return s.saveLocked()
}

// saveLocked assumes s.mu is held. Atomic: write .tmp then rename.
func (s *ConfigService) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		s.log.Error("create config dir", "path", s.path, "err", err)
		return err
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		s.log.Error("marshal config", "err", err)
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		s.log.Error("write temp config", "path", tmp, "err", err)
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		s.log.Error("rename temp config", "from", tmp, "to", s.path, "err", err)
		return err
	}
	s.log.Debug("config saved", "path", s.path, "bytes", len(data))
	return nil
}

func (c Config) clone() Config {
	out := c
	out.Profiles = make(map[string]ProfileConfig, len(c.Profiles))
	for k, v := range c.Profiles {
		v.EnabledOptional = append([]string(nil), v.EnabledOptional...)
		v.ExtraJVMArgs = append([]string(nil), v.ExtraJVMArgs...)
		out.Profiles[k] = v
	}
	return out
}
