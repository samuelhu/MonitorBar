package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"monitorbar/internal/display"
)

const arrowHeight = 10

// panel draws the store and reports the height it wants.
type panel struct {
	s *store
	// arrowX is where the arrow points, from the panel's left edge (macOS).
	arrowX float32
	// maxHeight is the work area the panel must fit; maxContent is derived
	// from it and the measured chrome on every frame.
	maxHeight  int
	maxContent float32
	// onHeight hears the panel's natural height once it changes.
	onHeight func(h int)

	height        int
	contentHeight float32
	headerHeight  float32
	footerHeight  float32
	settle        int  // frames still to draw while the sizes catch up
	ownFrame      bool // this frame was asked for by settle
}

func (p *panel) view(c *ui.Context) {
	p.updateMaxContent()
	// The frame starts with the theme of the system's appearance: keep that
	// appearance, and give the framework's widgets the panel's accent.
	c.SetTheme(widgetTheme(c))
	sk := skinOf(c)
	c.Root().Background(ui.Transparent) // the arrow and the rounded corners show the desktop
	shell := ui.Column(c).Width(panelWidth).TextColor(sk.ink)
	shell.DrawOver(func(pt *ui.Painter, r ui.Rect) {
		p.drawArrow(pt, r, sk)
	})
	shell.Children(func() {
		ui.Box(c).Height(arrowHeight)
		body := ui.Column(c).PaddingX(13).Gradient(sk.panelTop, sk.panelBottom, 135)
		body.Radius(16).Border(0.7, sk.panelStroke).Clip()
		var header, footer ui.Element
		body.Children(func() {
			header = p.header(c)
			p.content(c)
			footer = p.footer(c)
		})
		chromeChanged := false
		if h := header.Bounds().H; h > 0 && h != p.headerHeight {
			p.headerHeight = h
			chromeChanged = true
		}
		if h := footer.Bounds().H; h > 0 && h != p.footerHeight {
			p.footerHeight = h
			chromeChanged = true
		}
		if chromeChanged {
			p.updateMaxContent()
			p.settle = max(p.settle, 2)
			c.Invalidate()
		}
	})
	// Sizes come from the previous frame's layout. After a frame something
	// else asked for (input, a result), draw one more to measure it, and
	// more while the sizes keep changing.
	if !p.ownFrame {
		p.settle = max(p.settle, 1)
	}
	p.ownFrame = p.settle > 0
	if p.settle > 0 {
		p.settle--
		c.Invalidate()
	}
	if h := int(shell.Bounds().H + 0.5); h > 0 && h != p.height {
		p.height = h
		if p.onHeight != nil {
			go mygo.RunOnMain(func() { p.onHeight(h) }) // resize after this frame
		}
	}
}

func (p *panel) drawArrow(pt *ui.Painter, r ui.Rect, sk *skin) {
	x := min(max(p.arrowX, 22), panelWidth-22)
	left, tip, right := r.X+x-11, r.X+x, r.X+x+11
	base := r.Y + arrowHeight

	// Fill extends 1.2px into the body to seamlessly cover its top border under the arrow.
	var fill ui.Path
	fill.MoveTo(left, base+1.2)
	fill.QuadTo(tip-3, base-2, tip, r.Y+0.5)
	fill.QuadTo(tip+3, base-2, right, base+1.2)
	fill.Close()
	pt.FillPath(&fill, sk.panelTop)

	// Stroke only the two sloped sides so the body's 0.7px border continues smoothly over the tip.
	var edge ui.Path
	edge.MoveTo(left, base+0.35)
	edge.QuadTo(tip-3, base-2, tip, r.Y+0.5)
	edge.QuadTo(tip+3, base-2, right, base+0.35)
	pt.StrokePath(&edge, 0.7, sk.panelStroke)
}

func (p *panel) header(c *ui.Context) ui.Element {
	s := p.s
	sk := skinOf(c)
	return ui.Row(c).Gap(10).Padding(16, 0, 14, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(32, 32).Radius(9).Center().Gradient(sk.accent, sk.accentDeep, 180).Border(0.7, sk.iconBorder).Children(func() {
			icon(c, "display").FontSize(15).TextColor(sk.accentText)
		})
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, s.monitor.Name).FontSize(textTitle).FontWeight(weightSemibold).LetterSpacing(-0.2).SingleLine()
			ui.Text(c, p.caption()).FontSize(textSmall).FontWeight(weightRegular).TextColor(sk.muted).SingleLine()
		})
		more := ui.Box(c).Size(32, 32).Radius(16).Center().Cursor(ui.CursorPointer).Label(tr("更多", "More")).Tooltip(tr("更多", "More"))
		more.Background(sk.controlFill).Border(0.7, sk.cardStroke).TextColor(sk.muted)
		if more.Pressed() {
			more.Background(sk.controlPressed).TextColor(sk.ink)
		} else if more.Hovered() {
			more.Background(sk.controlHover).TextColor(sk.ink)
		}
		more.Children(func() {
			icon(c, "ellipsis").FontSize(15)
		}).Menu(func(m *ui.Menu) {
			if m.Item(tr("重新检测", "Rescan")).Chosen() {
				s.detect()
			}
			if m.Item(tr("系统显示器设置…", "System display settings…")).Chosen() {
				_ = mygo.Shell.OpenExternal(display.SettingsURL)
			}
			m.Separator()
			if m.Item(tr("登录时启动", "Launch at login")).Checked(s.launchAtLogin).Chosen() {
				s.toggleLaunchAtLogin()
			}
			m.Separator()
			if m.Item(tr("退出", "Quit")).Chosen() {
				mygo.App.Quit()
			}
		})
		power := ui.Box(c).Size(32, 32).Radius(16).Center().Cursor(ui.CursorPointer).Tooltip(tr("立即熄屏", "Sleep display")).Label(tr("立即熄屏", "Sleep display"))
		power.Background(sk.controlFill).Border(0.7, sk.cardStroke).TextColor(sk.muted)
		if power.Pressed() {
			power.Background(sk.controlPressed).TextColor(sk.ink)
		} else if power.Hovered() {
			power.Background(sk.controlHover).TextColor(sk.ink)
		}
		power.Children(func() { icon(c, "power").FontSize(14) })
		if power.Clicked() {
			s.sleepDisplay()
		}
	})
}

// caption sums the monitor up: "HDMI · 1920 × 1080 @ 160Hz".
func (p *panel) caption() string {
	if p.s.captionStr != "" {
		return p.s.captionStr
	}
	return p.s.computeCaption()
}

// content scrolls the panel when it is taller than the screen allows.
func (p *panel) content(c *ui.Context) {
	s := p.s
	sk := skinOf(c)
	scroll := ui.Scroll(c.Key("content")).Height(min(max(p.contentHeight, 1), p.maxContent))
	scroll.Children(func() {
		inner := ui.Column(c).Gap(10).Padding(0, 0, 4, 0)
		inner.Children(func() {
			p.overview(c)
			if s.notice != "" {
				banner := ui.Row(c).Gap(7).Padding(8, 10).Radius(9).AlignItems(ui.Center).
					Background(sk.bannerFill).Border(0.7, sk.cardStroke).
					Cursor(ui.CursorPointer)
				banner.Children(func() {
					ui.Box(c).Size(5, 5).Radius(2.5).Background(sk.accent)
					ui.Text(c, s.notice).FontSize(textCaption).TextColor(sk.ink).Grow(1)
				})
				if banner.Clicked() {
					s.setNotice("")
				}
			}
		})
		if h := inner.Bounds().H; h > 0 && h != p.contentHeight {
			p.contentHeight, p.settle = h, 3
			c.Invalidate()
		}
	})
}

// footer tells which channels reach the monitor. With none, it says
// nothing: software dimming needs no explaining.
func (p *panel) footer(c *ui.Context) ui.Element {
	s := p.s
	sk := skinOf(c)
	text := ""
	switch {
	case s.link != nil && s.ddc != nil:
		text = tr("已连接 · 局域网 + DDC/CI", "Connected · LAN + DDC/CI")
	case s.link != nil:
		text = tr("已通过局域网连接 · 澎湃 OS 遥控", "Connected over LAN · HyperOS remote")
	case s.ddc != nil:
		text = tr("已通过 DDC/CI 连接", "Connected over DDC/CI")
	case s.detectingAny():
		text = tr("正在检测显示器…", "Detecting display…")
	}
	if text == "" {
		return ui.Box(c).Height(12)
	}
	dot := sk.okDot
	if s.link == nil && s.ddc == nil {
		dot = sk.warnDot
	}
	return ui.Row(c).Gap(6).Padding(10, 0, 4, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		ui.Text(c, text).FontSize(textCaption).FontWeight(weightRegular).TextColor(sk.muted).Grow(1)
		if s.detectingAny() {
			ui.Spinner(c).Size(12, 12)
		}
	})
}

// chromeFallback is used until the first frame has measured header/footer.
const chromeFallback = 230

// chromeHeight is the panel height besides the scrolling content: arrow,
// header, footer and a margin from the screen's edge.
func (p *panel) chromeHeight() float32 {
	if p.headerHeight <= 0 || p.footerHeight <= 0 {
		return chromeFallback
	}
	return arrowHeight + p.headerHeight + p.footerHeight + 16
}

// updateMaxContent derives the scroll cap from the work area and the last
// measured chrome, so a growing footer (or a notice) cannot exceed the screen.
func (p *panel) updateMaxContent() {
	if p.maxHeight <= 0 {
		return
	}
	p.maxContent = max(float32(p.maxHeight)-p.chromeHeight(), 200)
}
