package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// settings are the choices kept between launches, in a JSON file.
type settings struct {
	SoftBrightness float64 `json:"softBrightness"` // gamma %, 20...100
	Warmth         float64 `json:"warmth"`         // 0...100
	Tab            string  `json:"tab"`
	NetworkHost    string  `json:"networkHost"` // the display found last time
	LaunchedBefore bool    `json:"launchedBefore"`

	// Switching to the day and night scenes at set times.
	AutoSwitch bool   `json:"autoSwitch"`
	DayStart   string `json:"dayStart"` // "07:00"
	NightStart string `json:"nightStart"`
	DayScene   string `json:"dayScene"`
	NightScene string `json:"nightScene"`
	AutoPeriod string `json:"autoPeriod"` // the period whose scene was applied last
	path       string
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
	s := &settings{SoftBrightness: 65, Warmth: 10, DayStart: "07:00", NightStart: "19:00", DayScene: "Default", NightScene: "Reading",
		path: filepath.Join(dataDir(), "settings.json")}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	if s.DayScene != "Default" && s.DayScene != "Reading" && s.DayScene != "Movie" {
		s.DayScene = "Default"
	}
	if s.NightScene != "Default" && s.NightScene != "Reading" && s.NightScene != "Movie" {
		s.NightScene = "Reading"
	}
	return s
}

func (s *settings) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o700)
	_ = os.WriteFile(s.path, data, 0o600)
}
