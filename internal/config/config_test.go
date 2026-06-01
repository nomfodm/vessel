package config

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFirstRunCreatesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	s := New(path, testLogger())
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := s.Snapshot()
	if got.Version != schemaVersion {
		t.Errorf("Version = %d, want %d", got.Version, schemaVersion)
	}
	if got.Profiles == nil {
		t.Error("Profiles is nil, want empty map")
	}

	if err := New(path, testLogger()).Load(); err != nil {
		t.Fatalf("file should exist after first run: %v", err)
	}
}

func TestSettersPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s := New(path, testLogger())
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if err := s.SetDataRoot(`D:\games`); err != nil {
		t.Fatalf("SetDataRoot: %v", err)
	}
	if err := s.SetLastProfile("survival"); err != nil {
		t.Fatalf("SetLastProfile: %v", err)
	}
	if err := s.SetProfileRAM("survival", 4096); err != nil {
		t.Fatalf("SetProfileRAM: %v", err)
	}
	if err := s.SetEnabledOptional("survival", []string{"optifine"}); err != nil {
		t.Fatalf("SetEnabledOptional: %v", err)
	}

	reloaded := New(path, testLogger())
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := reloaded.Snapshot()

	if got.DataRoot != `D:\games` {
		t.Errorf("DataRoot = %q, want %q", got.DataRoot, `D:\games`)
	}
	if got.LastProfile != "survival" {
		t.Errorf("LastProfile = %q, want survival", got.LastProfile)
	}
	p := got.Profiles["survival"]
	if p.RAMMB != 4096 {
		t.Errorf("RAMMB = %d, want 4096", p.RAMMB)
	}
	if len(p.EnabledOptional) != 1 || p.EnabledOptional[0] != "optifine" {
		t.Errorf("EnabledOptional = %v, want [optifine]", p.EnabledOptional)
	}
}

func TestSnapshotIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s := New(path, testLogger())
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.SetEnabledOptional("survival", []string{"optifine"}); err != nil {
		t.Fatalf("SetEnabledOptional: %v", err)
	}

	snap := s.Snapshot()
	snap.Profiles["survival"].EnabledOptional[0] = "mutated"
	snap.Profiles["hacked"] = ProfileConfig{RAMMB: 1}

	got := s.Snapshot()
	if got.Profiles["survival"].EnabledOptional[0] != "optifine" {
		t.Error("mutating snapshot slice leaked into service state")
	}
	if _, ok := got.Profiles["hacked"]; ok {
		t.Error("mutating snapshot map leaked into service state")
	}
}
