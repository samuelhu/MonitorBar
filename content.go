package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
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
				Tooltip(fmt.Sprintf("%s · 亮度 %d%% · %s", sc.name, int(sc.brightness), warmthLabel(sc.warmth)))
			e.Children(func() {
				icon(c, sc.icon).FontSize(15)
				ui.Text(c, sc.name).FontSize(12).FontWeight(600).LetterSpacing(-0.1)
				ui.Text(c, sc.tag).FontSize(10).FontWeight(400).TextColor(muted)
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
	groupLabel(c, "画面", s.pictureNote())
	card(c, func() {
		if s.hasBrightness {
			if (sliderRow{key: "backlight", title: "亮度", icon: "sun",
				value: &s.brightness, lo: 0, hi: 100, step: 5, unit: "%"}).show(c) {
				s.setBrightness(s.brightness)
			}
		} else if (sliderRow{key: "brightness", title: "亮度", icon: "sun",
			value: &s.prefs.SoftBrightness, lo: 20, hi: 100, step: 5, unit: "%"}).show(c) {
			s.applyGamma()
		}
		p.warmthRow(c)
	})
}

func (p *panel) warmthRow(c *ui.Context) {
	s := p.s
	if (sliderRow{key: "warmth", title: "冷暖", icon: "thermometer",
		value: &s.prefs.Warmth, lo: -50, hi: 100, step: 5, magneticZero: true,
		format: warmthLabel}).show(c) {
		s.applyGamma()
	}
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
	groupLabel(c, "音量", "")
	card(c, func() {
		if (sliderRow{key: "volume", title: "显示器音量", icon: "volume",
			value: &s.volume, lo: 0, hi: 100, step: 5, unit: "%"}).show(c) {
			s.setVolume(s.volume)
		}
	})
}

// routing is where the signal comes from and how fast it draws.
func (p *panel) routing(c *ui.Context) {
	s := p.s
	if sources := s.sources(); len(sources) > 0 {
		groupLabel(c, "输入源", s.inputChannel())
		card(c, func() {
			if len(sources) <= 4 {
				ui.Row(c).Gap(6).Children(func() {
					for _, src := range sources {
						if chipText(c.Key(src.ID), src.Name, s.input == src.ID) {
							s.selectInput(src)
						}
					}
				})
			} else {
				ui.Grid(c).Columns(4).Gap(6).Children(func() {
					for _, src := range sources {
						if chipText(c.Key(src.ID), src.Name, s.input == src.ID) {
							s.selectInput(src)
						}
					}
				})
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
	rates := s.cachedRates
	if len(rates) == 0 && len(s.modes) > 0 {
		rates = display.RefreshRates(s.modes, s.current)
	}
	// Keep standard desktop tiers (160, 144, 120, 60 or active) to form a neat single row
	var displayRates []display.Mode
	for _, m := range rates {
		if m.RefreshRate == 160 || m.RefreshRate == 144 || m.RefreshRate == 120 || m.RefreshRate == 60 || m.RefreshRate == rate {
			displayRates = append(displayRates, m)
		}
	}
	if len(displayRates) == 0 {
		displayRates = rates
	}
	if len(displayRates) > 1 {
		groupLabel(c, "刷新率", fmt.Sprintf("%d Hz", rate))
		card(c, func() {
			cols := min(len(displayRates), 4)
			if len(displayRates) <= 4 {
				ui.Row(c).Gap(6).Children(func() {
					for _, m := range displayRates {
						if chipText(c.Key(m.ID()), fmt.Sprintf("%d Hz", m.RefreshRate), m.RefreshRate == rate) {
							s.applyMode(m)
						}
					}
				})
			} else {
				ui.Grid(c).Columns(cols).Gap(6).Children(func() {
					for _, m := range displayRates {
						if chipText(c.Key(m.ID()), fmt.Sprintf("%d Hz", m.RefreshRate), m.RefreshRate == rate) {
							s.applyMode(m)
						}
					}
				})
			}
		})
	}
}

func (p *panel) ddcControls(c *ui.Context) {
	s := p.s
	if len(s.controls) == 0 {
		return
	}
	groupLabel(c, "显示器", "DDC/CI")
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
