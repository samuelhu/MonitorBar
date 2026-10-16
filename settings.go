package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// settings are the choices kept between launches, in a JSON file.
type settings struct {
	SoftBrightness float64 `json:"softBrightness"` // gamma %, 0...100
	Warmth         float64 `json:"warmth"`         // 0...100
	NetworkHost    string  `json:"networkHost"`    // the display found last time
	LastInput      int     `json:"lastInput"`      // the input the user picked
	HostInput      int     `json:"hostInput"`      // designated host connection port
	LaunchedBefore bool    `json:"launchedBefore"`
	path           string
}

// dataDir is where MonitorBar keeps its files.
func dataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("MonitorBar-%d", os.Getuid()))
	}
	return filepath.Join(dir, "MonitorBar")
}

func loadSettings() *settings {
	s := &settings{SoftBrightness: 100,
		path: filepath.Join(dataDir(), "settings.json")}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	s.normalize()
	return s
}

func (s *settings) normalize() {
	s.SoftBrightness = max(0, min(s.SoftBrightness, 100))
	s.Warmth = max(0, min(s.Warmth, 100))
	if s.LastInput < 0 || s.LastInput > 0xFFFF {
		s.LastInput = 0
	}
	if s.HostInput < 0 || s.HostInput > 0xFFFF {
		s.HostInput = 0
	}
}

func (s *settings) save() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path)
}
