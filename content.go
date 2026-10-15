package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// content is the panel body: scene presets, picture, output and routing.

func (p *panel) overview(c *ui.Context) {
	s := p.s
	p.scenes(c)
	p.picture(c)
	if s.hasVolume {
		p.volume(c)
	}
	p.routing(c)
	p.ddcControls(c)
}

// scenes is the hero row: three presets, applied in one click.
func (p *panel) scenes(c *ui.Context) {
	s := p.s
	ui.Row(c).Gap(6).Children(func() {
		for _, sc := range scenes {
			active := s.isActive(sc)
			e := chip(c.Key(sc.name), active).Grow(1).Height(58).Gap(3).
				Tooltip(fmt.Sprintf(tr("%s · 亮度 %d%% · %s", "%s · Brightness %d%% · %s"), sc.name, int(sc.brightness), warmthLabel(sc.warmth)))
			e.Children(func() {
				icon(c, sc.icon).FontSize(15)
				ui.Text(c, sc.name).FontSize(13).FontWeight(600).LetterSpacing(-0.1)
				ui.Text(c, sc.tag).FontSize(11).FontWeight(430).TextColor(muted)
			})
			if e.Clicked() {
				s.apply(sc)
			}
		}
	})
}

// picture groups the two software/hardware picture sliders.
func (p *panel) picture(c *ui.Context) {
	s := p.s
	groupLabel(c, tr("画面", "Picture"), s.pictureNote())
	card(c, func() {
		if s.hasBrightness {
			if (sliderRow{key: "backlight", title: tr("亮度", "Brightness"), icon: "sun",
				value: &s.brightness, lo: 0, hi: 100, step: 5, unit: "%"}).show(c) {
				s.setBrightness(s.brightness)
			}
		} else if (sliderRow{key: "brightness", title: tr("亮度", "Brightness"), icon: "sun",
			value: &s.prefs.SoftBrightness, lo: 20, hi: 100, step: 5, unit: "%"}).show(c) {
			s.applyGamma()
		}
		p.warmthRow(c)
	})
}

func (p *panel) warmthRow(c *ui.Context) {
	s := p.s
	if (sliderRow{key: "warmth", title: tr("冷暖", "Warmth"), icon: "thermometer",
		value: &s.prefs.Warmth, lo: -50, hi: 100, step: 5, magneticZero: true,
		format: warmthValue, unit: "K"}).show(c) {
		s.applyGamma()
	}
}

func warmthValue(v float64) string {
	return fmt.Sprint(warmthToKelvin(v))
}

func warmthLabel(v float64) string {
	return fmt.Sprintf("%dK", warmthToKelvin(v))
}

func warmthToKelvin(w float64) int {
	switch {
	case w == 0:
		return 6500
	case w == 10:
		return 5800
	case w == 25:
		return 4900
	}
	var m float64
	if w > 0 {
		m = 153.846 + w*1.85 + (w*w)*0.0031
	} else {
		m = 153.846 + w*0.926
	}
	k := 1e6 / m
	return int(math.Round(k/50.0)) * 50
}

func (p *panel) volume(c *ui.Context) {
	s := p.s
	groupLabel(c, tr("音量", "Volume"), muteNote(s))
	card(c, func() {
		if s.muted {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				mute := chip(c, true).Size(28, 24).Label(tr("恢复音量", "Restore volume")).Tooltip(tr("恢复音量", "Restore volume"))
				mute.Children(func() { icon(c, "volume-x").FontSize(13) })
				if mute.Clicked() {
					s.toggleMute()
				}
				ui.Text(c, tr("已静音", "Muted")).FontSize(12).FontWeight(600).LetterSpacing(-0.1).Grow(1)
				ui.Text(c, fmt.Sprintf(tr("原音量 %d%%", "Was %d%%"), int(s.returnTo))).FontSize(11).TextColor(muted)
			})
		} else if (sliderRow{key: "volume", title: tr("显示器音量", "Monitor volume"), icon: "volume",
			value: &s.volume, lo: 0, hi: 100, step: 5, unit: "%",
			muteToggle: true, onMute: s.toggleMute}).show(c) {
			s.setVolume(s.volume)
		}
	})
}

func muteNote(s *store) string {
	if s.muted {
		return tr("已静音", "Muted")
	}
	return ""
}

// routing is where the signal comes from and how fast it draws.
func (p *panel) routing(c *ui.Context) {
	s := p.s
	if sources := s.sources(); len(sources) > 0 {
		groupLabel(c, tr("输入源", "Input"), s.inputChannel())
		card(c, func() {
			chips := func() {
				for _, src := range sources {
					if chipText(c.Key(src.ID), src.Name, s.input == src.ID) {
						s.selectInput(src)
					}
				}
			}
			if len(sources) <= 4 {
				ui.Row(c).Gap(6).Children(chips)
			} else {
				ui.Grid(c).Columns(4).Gap(6).Children(chips)
			}
		})
	}
	p.refreshRates(c)
}

func (p *panel) refreshRates(c *ui.Context) {
	s := p.s
	rate := 60
	if s.hasCurrent {
		rate = s.current.RefreshRate
	}
	displayRates := s.cachedShowRates
	if len(displayRates) > 1 {
		groupLabel(c, tr("刷新率", "Refresh rate"), fmt.Sprintf("%d Hz", rate))
		card(c, func() {
			chips := func() {
				for _, m := range displayRates {
					if chipText(c.Key(m.ID()), fmt.Sprintf("%d Hz", m.RefreshRate), m.RefreshRate == rate) {
						s.applyMode(m)
					}
				}
			}
			if len(displayRates) <= 4 {
				ui.Row(c).Gap(6).Children(chips)
			} else {
				ui.Grid(c).Columns(min(len(displayRates), 4)).Gap(6).Children(chips)
			}
		})
	}
}

func (p *panel) ddcControls(c *ui.Context) {
	s := p.s
	if len(s.controls) == 0 {
		return
	}
	groupLabel(c, tr("显示器", "Monitor"), "DDC/CI")
	card(c, func() {
		for i := range s.controls {
			ctl := &s.controls[i]
			if (sliderRow{key: "ddc-" + ctl.name, title: ctl.name, icon: "sliders",
				value: &ctl.value, lo: 0, hi: float64(ctl.max), step: 1}).show(c) {
				s.setControl(i)
			}
		}
	})
}
