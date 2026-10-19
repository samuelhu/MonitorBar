package main

import (
	"testing"

	"github.com/egoist/mygo"
)

// The saved appearance choices map to mygo's sources and back, and
// unknown values fall back to following the system.
func TestThemeSourceMapping(t *testing.T) {
	for pref, want := range map[string]mygo.ThemeSource{
		themeSystem: mygo.ThemeSystem,
		themeLight:  mygo.ThemeLight,
		themeDark:   mygo.ThemeDark,
		"":          mygo.ThemeSystem,
		"sepia":     mygo.ThemeSystem,
	} {
		if got := themeSource(pref); got != want {
			t.Errorf("themeSource(%q) = %q, want %q", pref, got, want)
		}
	}
	for src, want := range map[mygo.ThemeSource]string{
		mygo.ThemeSystem: themeSystem,
		mygo.ThemeLight:  themeLight,
		mygo.ThemeDark:   themeDark,
		"":               themeSystem,
	} {
		if got := themePreference(src); got != want {
			t.Errorf("themePreference(%q) = %q, want %q", src, got, want)
		}
	}
}
