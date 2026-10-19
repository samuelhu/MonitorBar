package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The saved appearance choices: follow the system's light/dark mode, or
// pin one.
const (
	themeSystem = "system"
	themeLight  = "light"
	themeDark   = "dark"
)

// themeSource maps a saved appearance choice to mygo's appearance source.
func themeSource(pref string) mygo.ThemeSource {
	switch pref {
	case themeLight:
		return mygo.ThemeLight
	case themeDark:
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

// themePreference is the saved form of an appearance source.
func themePreference(src mygo.ThemeSource) string {
	switch src {
	case mygo.ThemeLight:
		return themeLight
	case mygo.ThemeDark:
		return themeDark
	default:
		return themeSystem
	}
}

// skin is the palette of one appearance. The system's light or dark mode
// picks it (see skinOf); the panel background, the cards and the text on
// them, the chips and the accented bits all come from here.
type skin struct {
	// The panel and its hairline.
	panelTop, panelBottom ui.Color
	panelStroke           ui.Color

	// Text: primary, and the quiet labels over cards and groups.
	ink, muted ui.Color

	// Cards: the surface a group of controls sits in.
	cardFill, cardStroke ui.Color

	// Chips: scene presets, inputs and refresh rates.
	chipFill, chipStroke           ui.Color
	chipHover, chipHoverStroke     ui.Color
	chipPressed, chipPressedStroke ui.Color

	// The round buttons in the header (More, Sleep display).
	controlFill, controlHover, controlPressed ui.Color

	// The notice banner, and the outline of the input that is this Mac's
	// own connection.
	bannerFill, hostBorder ui.Color

	// The Xiaomi orange: fills, the accent-colored text that has to read
	// on the panel, and text on accent fills.
	accent, accentDeep         ui.Color
	accentInk, accentText      ui.Color
	accentHover, accentPressed ui.Color
	// The rim of the gradient monitor badge in the header.
	iconBorder ui.Color

	// The footer's state dots.
	okDot, warnDot ui.Color
}

// darkSkin keeps the panel's original night look: a near-black sheet,
// frosted white cards and the full-strength orange.
var darkSkin = &skin{
	panelTop:    ui.RGB(18, 19, 21),
	panelBottom: ui.RGB(10, 10, 10),
	panelStroke: ui.RGBA(255, 255, 255, 0.12),

	ink:   ui.RGBA(255, 255, 255, 0.87),
	muted: ui.RGBA(255, 255, 255, 0.52),

	cardFill:   ui.RGBA(255, 255, 255, 0.055),
	cardStroke: ui.RGBA(255, 255, 255, 0.07),

	chipFill:          ui.RGBA(255, 255, 255, 0.06),
	chipStroke:        ui.RGBA(255, 255, 255, 0.05),
	chipHover:         ui.RGBA(255, 255, 255, 0.10),
	chipHoverStroke:   ui.RGBA(255, 255, 255, 0.11),
	chipPressed:       ui.RGBA(255, 255, 255, 0.14),
	chipPressedStroke: ui.RGBA(255, 255, 255, 0.14),

	controlFill:    ui.RGBA(255, 255, 255, 0.06),
	controlHover:   ui.RGBA(255, 255, 255, 0.11),
	controlPressed: ui.RGBA(255, 255, 255, 0.16),

	bannerFill: ui.RGBA(255, 255, 255, 0.055),
	hostBorder: ui.RGBA(255, 255, 255, 0.16),

	accent:        ui.RGB(255, 105, 0),
	accentDeep:    ui.RGB(255, 70, 0),
	accentInk:     ui.RGB(255, 122, 26),
	accentText:    ui.RGB(255, 255, 255),
	accentHover:   ui.RGB(255, 127, 42),
	accentPressed: ui.RGB(230, 92, 0),
	iconBorder:    ui.RGBA(255, 255, 255, 0.22),

	okDot:   ui.RGB(52, 199, 89),
	warnDot: ui.RGB(255, 204, 0),
}

// lightSkin mirrors the dark one in daylight: a soft grey sheet, white
// cards, and the orange darkened just enough to read as text on white.
var lightSkin = &skin{
	panelTop:    ui.RGB(251, 251, 253),
	panelBottom: ui.RGB(241, 241, 245),
	panelStroke: ui.RGBA(0, 0, 0, 0.14),

	ink:   ui.RGBA(24, 24, 28, 0.92),
	muted: ui.RGBA(60, 60, 67, 0.66),

	cardFill:   ui.RGBA(255, 255, 255, 0.72),
	cardStroke: ui.RGBA(15, 15, 20, 0.06),

	chipFill:          ui.RGBA(15, 15, 20, 0.045),
	chipStroke:        ui.RGBA(15, 15, 20, 0.035),
	chipHover:         ui.RGBA(15, 15, 20, 0.075),
	chipHoverStroke:   ui.RGBA(15, 15, 20, 0.065),
	chipPressed:       ui.RGBA(15, 15, 20, 0.11),
	chipPressedStroke: ui.RGBA(15, 15, 20, 0.10),

	controlFill:    ui.RGBA(15, 15, 20, 0.045),
	controlHover:   ui.RGBA(15, 15, 20, 0.08),
	controlPressed: ui.RGBA(15, 15, 20, 0.12),

	bannerFill: ui.RGBA(15, 15, 20, 0.04),
	hostBorder: ui.RGBA(15, 15, 20, 0.16),

	accent:        ui.RGB(255, 105, 0),
	accentDeep:    ui.RGB(230, 74, 0),
	accentInk:     ui.RGB(194, 65, 12),
	accentText:    ui.RGB(255, 255, 255),
	accentHover:   ui.RGB(242, 91, 0),
	accentPressed: ui.RGB(194, 65, 12),
	iconBorder:    ui.RGBA(255, 255, 255, 0.22),

	okDot:   ui.RGB(31, 157, 69),
	warnDot: ui.RGB(191, 122, 0),
}

// skinOf returns the palette of the appearance this frame follows: the
// system's light or dark mode, unless mygo.Theme.SetSource forced one.
func skinOf(c *ui.Context) *skin {
	if c.Theme().Dark {
		return darkSkin
	}
	return lightSkin
}

// widgetTheme dresses the framework's sliders and menus in the appearance
// of the frame, with the app's type and the Xiaomi orange.
func widgetTheme(c *ui.Context) *ui.Theme {
	t := *c.Theme() // the system's theme for this frame, preferences included
	sk := skinOf(c)
	t.Accent, t.AccentHover, t.AccentPressed = sk.accent, sk.accentHover, sk.accentPressed
	t.AccentText = sk.accentText
	t.Font = "system-ui" // SF Pro on macOS
	t.FontSize = 12
	t.Selection = sk.accent.Alpha(0.28)
	t.Focus = sk.accent.Alpha(0.55)
	return &t
}
