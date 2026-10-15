package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Line icons in the style of Lucide, drawn in the text color.
var icons = map[string]*ui.SVG{}

func init() {
	for name, body := range map[string]string{
		"display":     `<rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/>`,
		"sliders":     `<path d="M21 4h-7"/><path d="M10 4H3"/><path d="M21 12h-9"/><path d="M8 12H3"/><path d="M21 20h-5"/><path d="M12 20H3"/><path d="M14 2v4"/><path d="M8 10v4"/><path d="M16 18v4"/>`,
		"aspect":      `<rect width="12" height="20" x="6" y="2" rx="2"/><rect width="20" height="12" x="2" y="6" rx="2"/>`,
		"chart":       `<path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/>`,
		"sun":         `<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>`,
		"code":        `<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>`,
		"book":        `<path d="M2 4h6a4 4 0 0 1 4 4v13a3 3 0 0 0-3-3H2z"/><path d="M22 4h-6a4 4 0 0 0-4 4v13a3 3 0 0 1 3-3h7z"/>`,
		"film":        `<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 3v18"/><path d="M17 3v18"/><path d="M3 12h18"/><path d="M3 7.5h4"/><path d="M3 16.5h4"/><path d="M17 7.5h4"/><path d="M17 16.5h4"/>`,
		"moon":        `<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z"/><path d="M19 3v4"/><path d="M21 5h-4"/>`,
		"sparkles":    `<path d="M11 3l1.9 5.6L18.5 10.5l-5.6 1.9L11 18l-1.9-5.6L3.5 10.5l5.6-1.9z"/><path d="M19 15v4"/><path d="M21 17h-4"/>`,
		"power":       `<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/>`,
		"ellipsis":    `<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>`,
		"volume":      `<path d="M11 5 6 9H2v6h4l5 4z"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>`,
		"plug":        `<path d="M12 22v-5"/><path d="M9 8V2"/><path d="M15 8V2"/><path d="M18 8v5a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V8z"/>`,
		"gauge":       `<path d="m12 14 4-4"/><path d="M3.34 19a10 10 0 1 1 17.32 0"/>`,
		"clock":       `<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>`,
		"calendar":    `<rect width="18" height="18" x="3" y="4" rx="2"/><path d="M16 2v4"/><path d="M8 2v4"/><path d="M3 10h18"/>`,
		"keyboard":    `<rect width="20" height="16" x="2" y="4" rx="2"/><path d="M6 8h.01"/><path d="M10 8h.01"/><path d="M14 8h.01"/><path d="M18 8h.01"/><path d="M8 12h.01"/><path d="M12 12h.01"/><path d="M16 12h.01"/><path d="M7 16h10"/>`,
		"thermometer": `<path d="M14 4v10.54a4 4 0 1 1-4 0V4a2 2 0 0 1 4 0z"/>`,
		"minus":       `<path d="M5 12h14"/>`,
		"plus":        `<path d="M5 12h14"/><path d="M12 5v14"/>`,
		"check":       `<path d="M20 6 9 17l-5-5"/>`,
		"rotate":      `<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>`,
		"contrast":    `<circle cx="12" cy="12" r="10"/><path d="M12 18a6 6 0 0 0 0-12z"/>`,
		"zoom":        `<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/><path d="M11 8v6"/><path d="M8 11h6"/>`,
		"layers":      `<rect width="14" height="14" x="8" y="8" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,
	} {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
	}
}

// distToSegment calculates Euclidean distance from (px, py) to segment (x1, y1)-(x2, y2).
func distToSegment(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-x1, py-y1)
	}
	t := max(0.0, min(1.0, ((px-x1)*dx+(py-y1)*dy)/l2))
	projX := x1 + t*dx
	projY := y1 + t*dy
	return math.Hypot(px-projX, py-projY)
}

// trayIcon draws a modern SF-Symbols styled monitor template icon.
// On macOS Retina displays, a 36x36 px PNG backing an 18x18 pt canvas
// renders with mathematically precise subpixel antialiasing and smooth rounded corners.
func trayIcon() []byte {
	const (
		scale = 2.0 // @2x Retina: 36x36 pixels for 18x18 points
		size  = 36
	)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	// Proportions tuned to Apple SF Symbols 'display' / 'desktopcomputer' (18x18 pt canvas)
	cx, cy := 9.0, 6.6
	hw, hh := 7.5, 5.0
	r := 2.0
	s := 1.25 // SF Symbols Medium stroke width

	ihw := hw - s
	ihh := hh - s
	ir := math.Max(r-s, 0.6)

	inkRGB := uint8(0)
	if runtime.GOOS != "darwin" {
		inkRGB = 240
	}

	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			x := (float64(px) + 0.5) / scale
			y := (float64(py) + 0.5) / scale

			// Outer rounded rectangle
			dx := math.Abs(x-cx) - (hw - r)
			dy := math.Abs(y-cy) - (hh - r)
			dOuter := math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - r

			// Inner cutout rounded rectangle
			idx := math.Abs(x-cx) - (ihw - ir)
			idy := math.Abs(y-cy) - (ihh - ir)
			dInner := math.Hypot(math.Max(idx, 0), math.Max(idy, 0)) + math.Min(math.Max(idx, idy), 0) - ir

			// Screen frame boundary
			dScreen := math.Max(dOuter, -dInner)

			// Stand neck
			dNeck := distToSegment(x, y, 9.0, 11.5, 9.0, 14.8) - s/2

			// Stand base with rounded caps
			dBase := distToSegment(x, y, 4.8, 14.8, 13.2, 14.8) - s/2

			d := math.Min(dScreen, math.Min(dNeck, dBase))

			// Analytic subpixel antialiasing
			dPx := d * scale
			coverage := math.Max(0, math.Min(1, 0.5-dPx))

			if coverage > 0 {
				a := uint8(math.Round(coverage * 255))
				img.SetNRGBA(px, py, color.NRGBA{R: inkRGB, G: inkRGB, B: inkRGB, A: a})
			}
		}
	}

	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
