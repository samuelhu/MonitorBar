package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsWritesSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := &settings{SoftBrightness: 80, Warmth: 10, path: path}
	s.normalize()
	if s.Version != settingsVersion {
		t.Fatalf("normalize set version %d, want %d", s.Version, settingsVersion)
	}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Version != settingsVersion {
		t.Fatalf("saved version %d, want %d", raw.Version, settingsVersion)
	}
}

func TestSettingsNormalizeUpgradesVersion(t *testing.T) {
	s := &settings{}
	s.normalize()
	if s.Version != settingsVersion {
		t.Fatalf("version %d not upgraded to %d", s.Version, settingsVersion)
	}
}
