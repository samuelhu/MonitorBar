// Package display reads and changes the target monitor through macOS: its
// modes, its gamma table and whether it is lit. display_darwin.go implements
// Target and the Monitor methods.
package display

import (
	"fmt"
	"slices"
)

// Monitor is the external display the app manages.
type Monitor struct {
	Name       string // e.g. "Mi Monitor"
	Connection string // e.g. "HDMI", or "" when unknown
	handle     handle
}

// NativeSize is the panel's own resolution: the largest mode drawn 1:1, as
// HiDPI modes draw more pixels than the panel has. The current mode wins if
// it is larger (virtual displays of remote desktops can report a smaller
// native size than the mode in use).
func (m Monitor) NativeSize(modes []Mode, current Mode) (w, h int) {
	for _, mode := range modes {
		if !mode.HiDPI() && mode.PixelWidth > w {
			w, h = mode.PixelWidth, mode.PixelHeight
		}
	}
	if !current.HiDPI() && current.PixelWidth > w {
		w, h = current.PixelWidth, current.PixelHeight
	}
	return w, h
}

// SizeLabel names a resolution the way monitors are sold: "4K", "2K".
func SizeLabel(w, h int) string {
	switch {
	case w >= 5120:
		return "5K"
	case w >= 3840:
		return "4K"
	case h >= 1440:
		return "2K"
	case h > 0:
		return fmt.Sprintf("%dp", h)
	}
	return ""
}

// Mode is a resolution and refresh rate the monitor can show.
type Mode struct {
	Width, Height           int // the size the desktop uses (points on macOS)
	PixelWidth, PixelHeight int // the size drawn
	RefreshRate             int
	sizeKey                 string
	id                      string
	native                  nativeMode
}

func (m Mode) HiDPI() bool { return m.PixelWidth > m.Width }

// SizeKey identifies the resolution, whatever the refresh rate.
func (m Mode) SizeKey() string {
	if m.sizeKey != "" {
		return m.sizeKey
	}
	return fmt.Sprintf("%dx%d@%v", m.Width, m.Height, m.HiDPI())
}

func (m Mode) ID() string {
	if m.id != "" {
		return m.id
	}
	return fmt.Sprintf("%s/%d", m.SizeKey(), m.RefreshRate)
}
func (m Mode) Title() string { return fmt.Sprintf("%d × %d", m.Width, m.Height) }

// WithoutHandle returns a copy of m with its platform-native handle cleared,
// so derived UI caches never alias or retain native handles owned by Modes().
func (m Mode) WithoutHandle() Mode {
	var zero nativeMode
	m.native = zero
	return m
}

// RefreshRates returns the modes of the resolution of current, fastest first.
func RefreshRates(modes []Mode, current Mode) []Mode {
	var out []Mode
	for _, m := range modes {
		if m.SizeKey() == current.SizeKey() {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b Mode) int { return b.RefreshRate - a.RefreshRate })
	return out
}

// dedupe keeps the first mode of each ID: systems list the same mode once
// per pixel encoding. Duplicate native handles are released immediately.
func dedupe(modes []Mode) []Mode {
	seen := map[string]bool{}
	return slices.DeleteFunc(modes, func(m Mode) bool {
		if seen[m.ID()] {
			m.Release()
			return true
		}
		seen[m.ID()] = true
		return false
	})
}

// gains scales the color channels in gamma-encoded LUT space with a linear
// safe floor [0.12..1.0] so 0% never blacks out the display or creates a
// bottom dead zone: warmth in [0..1] follows a smooth Planckian-inspired
// locus (cutting green gently and blue more).
func gains(brightness, warmth float64) (r, g, b float64) {
	gain := max(0.0, min(brightness, 1.0))
	gain = 0.12 + 0.88*gain
	w := max(0.0, min(warmth, 1.0))
	green := 1 - (0.16*w + 0.06*w*w)
	blue := 1 - 0.55*w
	return gain, gain * green, gain * blue
}

// neutral reports settings that leave the display untouched.
func neutral(brightness, warmth float64) bool {
	return brightness >= 0.999 && warmth <= 0.001
}

// lutMatches reports whether two 3-channel LUTs have identical lengths and
// every entry differs by at most eps.
func lutMatches(a, b [3][]float32, eps float32) bool {
	n := len(a[0])
	if n == 0 || len(b[0]) != n {
		return false
	}
	for ch := 0; ch < 3; ch++ {
		if len(a[ch]) != n || len(b[ch]) != n {
			return false
		}
		ac, bc := a[ch], b[ch]
		for i := 0; i < n; i++ {
			diff := ac[i] - bc[i]
			if diff < -eps || diff > eps {
				return false
			}
		}
	}
	return true
}
