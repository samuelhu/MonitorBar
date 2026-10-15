package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
)

// MARK: 主面板 (精简极致整合视图，零装饰物，无多余滚动)

func (p *panel) overview(c *ui.Context) {
	s := p.s
	p.scenes(c)
	if sources := s.sources(); len(sources) > 0 {
		card(c, func() {
			cardTitle(c, "输入源", "plug", s.inputChannel())
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
	if s.hasBrightness {
		if (sliderCard{key: "backlight", title: "亮度", caption: "硬件背光 · DDC/CI", icon: "sun",
			value: &s.brightness, lo: 0, hi: 100, step: 5}).show(c) {
			s.setBrightness(s.brightness)
		}
	} else if (sliderCard{key: "brightness", title: "亮度", caption: "伽马感知补偿 · 护眼线性", icon: "sun",
		value: &s.prefs.SoftBrightness, lo: 20, hi: 100, step: 5}).show(c) {
		s.applyGamma()
	}
	p.warmthCard(c)
	if s.hasVolume {
		p.volume(c)
	}
	p.refreshRates(c)
	p.ddcControls(c)
}

func (p *panel) volume(c *ui.Context) {
	s := p.s
	caption := "局域网 · 澎湃 OS"
	if s.link == nil && s.ddc != nil {
		caption = "硬件 · DDC/CI"
	}
	if (sliderCard{key: "volume", title: "显示器音量", caption: caption, icon: "volume",
		value: &s.volume, lo: 0, hi: 100, step: 5}).show(c) {
		s.setVolume(s.volume)
	}
}

func (p *panel) warmthCard(c *ui.Context) {
	s := p.s
	caption := "标准 D65 · 普朗克黑体轨迹"
	w := math.Round(s.prefs.Warmth)
	switch {
	case w > 0:
		caption = fmt.Sprintf("暖色微调 (+%d) · 普朗克黑体轨迹", int(w))
	case w < 0:
		caption = fmt.Sprintf("冷色微调 (%d) · 普朗克黑体轨迹", int(w))
	}
	if (sliderCard{key: "warmth", title: "冷暖", caption: caption, icon: "thermometer",
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

func (p *panel) scenes(c *ui.Context) {
	s := p.s
	card(c, func() {
		cardTitle(c, "场景模式", "sparkles", "科学护眼预设")
		ui.Row(c).Gap(6).Children(func() {
			for _, sc := range scenes {
				active := s.isActive(sc)
				e := chip(c.Key(sc.name), active).Grow(1).Height(54).Gap(2).
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
	})
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
		card(c, func() {
			cardTitle(c, "刷新率", "gauge", fmt.Sprintf("当前 %d Hz", rate))
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
	for i := range s.controls {
		ctl := &s.controls[i]
		if (sliderCard{key: "ddc-" + ctl.name, title: ctl.name, caption: "硬件 · DDC/CI", icon: "sliders",
			value: &ctl.value, lo: 0, hi: float64(ctl.max), step: 1}).show(c) {
			s.setControl(i)
		}
	}
}
