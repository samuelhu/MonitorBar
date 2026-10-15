package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

const panelWidth = 360

var (
	accent      = ui.RGB(255, 105, 0) // Xiaomi orange
	accentDeep  = ui.RGB(255, 69, 0)
	ink         = ui.RGBA(255, 255, 255, 0.87)
	muted       = ui.RGBA(255, 255, 255, 0.5)
	cardFill    = ui.RGBA(255, 255, 255, 0.055)
	cardStroke  = ui.RGBA(255, 255, 255, 0.07)
	panelTop    = ui.RGB(18, 19, 21)
	panelBottom = ui.RGB(10, 10, 10)
	darkTheme   = func() *ui.Theme {
		t := *ui.DarkTheme()
		t.Accent, t.AccentHover, t.AccentPressed = accent, accent, accentDeep
		return &t
	}()
)

func icon(c *ui.Context, name string) ui.Element { return ui.Icon(c, icons[name]) }

// card is the rounded surface a group of controls sits in.
func card(c *ui.Context, children func()) ui.Element {
	return ui.Column(c).Padding(12).Gap(10).Radius(12).Background(cardFill).Border(0.7, cardStroke).Children(children)
}

// groupLabel is the quiet caption above a card: one short word, optional note.
func groupLabel(c *ui.Context, title, trailing string) {
	ui.Row(c).Gap(6).Padding(0, 2, 0, 2).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(muted).Grow(1)
		if trailing != "" {
			ui.Text(c, trailing).FontSize(11).FontWeight(400).TextColor(muted)
		}
	})
}

// chip is a pressable tile; selected ones take the accent.
func chip(c *ui.Context, selected bool) ui.Element {
	e := ui.Box(c).Radius(9).Center().Cursor(ui.CursorPointer)
	switch {
	case selected:
		e.Background(accent.Alpha(0.18)).Border(0.8, accent.Alpha(0.55)).TextColor(accent)
	case e.Pressed():
		e.Background(ui.RGBA(255, 255, 255, 0.13)).TextColor(ink)
	case e.Hovered():
		e.Background(ui.RGBA(255, 255, 255, 0.1)).TextColor(ink)
	default:
		e.Background(ui.RGBA(255, 255, 255, 0.06)).TextColor(ink)
	}
	return e
}

// chipText is a chip with one line of text, as the input and refresh-rate
// buttons.
func chipText(c *ui.Context, label string, selected bool) bool {
	e := chip(c, selected).Grow(1).Height(32)
	e.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(500).LetterSpacing(-0.2).SingleLine() })
	return e.Clicked()
}

// sliderRow is one control line: name, large value, then − slider +.
type sliderRow struct {
	key, title, icon string
	value            *float64
	lo, hi, step     float64
	magneticZero     bool
	format           func(float64) string // the value as shown, default an integer
	unit             string               // small suffix next to the number
}

// show draws it and reports whether the user changed the value.
func (s sliderRow) show(c *ui.Context) bool {
	changed := false
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			icon(c, s.icon).FontSize(14).TextColor(muted).Width(18)
			ui.Text(c, s.title).FontSize(12).FontWeight(600).LetterSpacing(-0.1).Grow(1)
			text := fmt.Sprint(int(math.Round(*s.value)))
			if s.format != nil {
				text = s.format(*s.value)
			}
			ui.Text(c, text).FontSize(17).FontWeight(600).LetterSpacing(-0.3).FontFeatures("tnum")
			if s.unit != "" {
				ui.Text(c, s.unit).FontSize(11).FontWeight(500).TextColor(muted)
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if stepButton(c, "minus") {
				v := math.Round(*s.value - s.step)
				if s.magneticZero && math.Abs(v) < s.step/2 {
					v = 0
				}
				*s.value, changed = max(v, s.lo), true
			}
			if ui.Slider(c.Key(s.key), s.value, s.lo, s.hi).Label(s.title).Grow(1).Changed() {
				if s.magneticZero && math.Abs(*s.value) <= 2.0 {
					*s.value = 0
				} else {
					*s.value = math.Round(*s.value)
				}
				changed = true
			}
			if stepButton(c, "plus") {
				v := math.Round(*s.value + s.step)
				if s.magneticZero && math.Abs(v) < s.step/2 {
					v = 0
				}
				*s.value, changed = min(v, s.hi), true
			}
		})
	})
	return changed
}

func stepButton(c *ui.Context, name string) bool {
	e := chip(c, false).Size(28, 24)
	e.Children(func() { icon(c, name).FontSize(12) })
	return e.Clicked()
}
