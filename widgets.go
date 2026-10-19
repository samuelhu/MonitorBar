package main

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
)

const panelWidth = 360

// One type scale for the panel: captions, small labels, body text, the
// monitor title, and the large numeric read-outs.
const (
	textCaption float32 = 11
	textSmall   float32 = 12
	textBody    float32 = 13
	textTitle   float32 = 16
	textValue   float32 = 18

	weightRegular  = 400
	weightMedium   = 500
	weightSemibold = 600
)

func icon(c *ui.Context, name string) ui.Element { return ui.Icon(c, icons[name]) }

// card is the rounded surface a group of controls sits in.
func card(c *ui.Context, children func()) ui.Element {
	sk := skinOf(c)
	return ui.Column(c).Padding(12).Gap(10).Radius(12).Background(sk.cardFill).Border(0.7, sk.cardStroke).Children(children)
}

// groupLabel is the quiet caption above a card: one short word, optional note.
func groupLabel(c *ui.Context, title, trailing string) {
	sk := skinOf(c)
	ui.Row(c).Gap(6).Padding(0, 2, 0, 2).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(textCaption).FontWeight(weightSemibold).LetterSpacing(0.2).TextColor(sk.muted).Grow(1)
		if trailing != "" {
			ui.Text(c, trailing).FontSize(textCaption).FontWeight(weightRegular).TextColor(sk.muted)
		}
	})
}

// chip is a pressable tile; selected ones take the accent.
func chip(c *ui.Context, selected bool) ui.Element {
	sk := skinOf(c)
	e := ui.Box(c).Radius(9).Center().Cursor(ui.CursorPointer)
	switch {
	case selected:
		e.Background(sk.accent.Alpha(0.18)).Border(0.8, sk.accent.Alpha(0.55)).TextColor(sk.accentInk)
	case e.Pressed():
		e.Background(sk.chipPressed).Border(0.7, sk.chipPressedStroke).TextColor(sk.ink)
	case e.Hovered():
		e.Background(sk.chipHover).Border(0.7, sk.chipHoverStroke).TextColor(sk.ink)
	default:
		e.Background(sk.chipFill).Border(0.7, sk.chipStroke).TextColor(sk.ink)
	}
	return e
}

// chipText is a chip with one line of text, as the input and refresh-rate
// buttons.
func chipText(c *ui.Context, label string, selected bool) bool {
	e := chip(c, selected).Grow(1).Height(32)
	weight := weightMedium
	if selected {
		weight = weightSemibold
	}
	e.Children(func() { ui.Text(c, label).FontSize(textBody).FontWeight(weight).LetterSpacing(-0.1).SingleLine() })
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
	onMute           func()               // called when the sound icon is clicked
	isMuted          bool
}

// show draws it and reports whether the user changed the value.
func (s sliderRow) show(c *ui.Context) bool {
	sk := skinOf(c)
	changed := false
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if s.onMute != nil {
				muteBtn := ui.Box(c).Size(22, 22).Radius(6).Center().Cursor(ui.CursorPointer)
				tooltip := tr("静音", "Mute")
				if s.isMuted {
					tooltip = tr("恢复音量", "Restore volume")
					bg := sk.accent.Alpha(0.18)
					if muteBtn.Hovered() {
						bg = sk.accent.Alpha(0.28)
					}
					muteBtn.Background(bg).TextColor(sk.accentInk)
				} else if muteBtn.Hovered() {
					muteBtn.Background(sk.controlHover).TextColor(sk.ink)
				} else {
					muteBtn.TextColor(sk.muted)
				}
				muteBtn.Tooltip(tooltip).Label(tooltip)
				muteBtn.Children(func() {
					icon(c, s.icon).FontSize(14)
				})
				if muteBtn.Clicked() {
					s.onMute()
				}
			} else {
				ui.Box(c).Size(22, 22).Center().Children(func() {
					icon(c, s.icon).FontSize(14).TextColor(sk.muted)
				})
			}
			ui.Text(c, s.title).FontSize(textBody).FontWeight(weightSemibold).LetterSpacing(-0.1).Grow(1)
			text := strconv.Itoa(int(math.Round(*s.value)))
			if s.format != nil {
				text = s.format(*s.value)
			}
			numColor := sk.ink
			if s.isMuted {
				numColor = sk.muted
			}
			ui.Row(c).Gap(2.5).AlignItems(ui.Center).Children(func() {
				ui.Text(c, text).FontSize(textValue).FontWeight(weightSemibold).LetterSpacing(-0.2).FontFeatures("tnum").TextColor(numColor)
				if s.unit != "" {
					ui.Text(c, s.unit).FontSize(textCaption).FontWeight(weightMedium).TextColor(sk.muted)
				}
			})
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if stepButton(c, "minus", tr("减小", "Decrease ")+s.title) {
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
			if stepButton(c, "plus", tr("增大", "Increase ")+s.title) {
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

func stepButton(c *ui.Context, name, label string) bool {
	sk := skinOf(c)
	e := chip(c, false).Size(28, 24).Label(label)
	icColor := sk.muted
	if e.Hovered() || e.Pressed() {
		icColor = sk.ink
	}
	e.Children(func() { icon(c, name).FontSize(12).TextColor(icColor) })
	return e.Clicked()
}
