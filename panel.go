package main

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
)

const arrowHeight = 10

// panel draws the store and reports the height it wants.
type panel struct {
	s *store
	// arrowX is where the arrow points, from the panel's left edge (macOS).
	arrowX float32
	// maxContent caps the scrolling area, so the panel fits the screen.
	maxContent float32
	// onHeight hears the panel's natural height once it changes.
	onHeight func(h int)

	height        int
	contentHeight float32
	settle        int  // frames still to draw while the sizes catch up
	ownFrame      bool // this frame was asked for by settle
}

func (p *panel) view(c *ui.Context) {
	c.SetTheme(darkTheme)
	if runtime.GOOS == "windows" {
		// An opaque root lets text use ClearType: subpixel glyphs need a
		// solid background. Windows 11 rounds and outlines the flyout itself.
		c.Root().Background(panelBottom)
	} else {
		c.Root().Background(ui.Transparent) // the arrow and the rounded corners show the desktop
	}
	shell := ui.Column(c).Width(panelWidth).TextColor(ink)
	if runtime.GOOS == "darwin" {
		shell.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			p.drawArrow(pt, r)
		})
	}
	shell.Children(func() {
		if runtime.GOOS == "darwin" {
			ui.Box(c).Height(arrowHeight)
		}
		body := ui.Column(c).PaddingX(13).Gradient(panelTop, panelBottom, 135)
		if runtime.GOOS == "darwin" { // Windows 11 rounds and outlines the flyout itself
			body.Radius(16).Border(0.7, ui.RGBA(255, 255, 255, 0.12)).Clip()
		}
		body.Children(func() {
			p.header(c)
			p.content(c)
			p.footer(c)
		})
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

func (p *panel) drawArrow(pt *ui.Painter, r ui.Rect) {
	x := min(max(p.arrowX, 22), panelWidth-22)
	left, tip, right := r.X+x-11, r.X+x, r.X+x+11
	base := r.Y + arrowHeight

	// Fill extends 1.2px into the body to seamlessly cover its top border under the arrow.
	var fill ui.Path
	fill.MoveTo(left, base+1.2)
	fill.QuadTo(tip-3, base-2, tip, r.Y+0.5)
	fill.QuadTo(tip+3, base-2, right, base+1.2)
	fill.Close()
	pt.FillPath(&fill, panelTop)

	// Stroke only the two sloped sides so the body's 0.7px border continues smoothly over the tip.
	var edge ui.Path
	edge.MoveTo(left, base+0.35)
	edge.QuadTo(tip-3, base-2, tip, r.Y+0.5)
	edge.QuadTo(tip+3, base-2, right, base+0.35)
	pt.StrokePath(&edge, 0.7, ui.RGBA(255, 255, 255, 0.12))
}

func (p *panel) header(c *ui.Context) {
	s := p.s
	ui.Row(c).Gap(10).Padding(16, 0, 14, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(32, 32).Radius(9).Center().Gradient(accent, accentDeep, 180).Border(0.7, ui.RGBA(255, 255, 255, 0.22)).Children(func() {
			icon(c, "display").FontSize(15).TextColor(ui.RGB(255, 255, 255))
		})
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, s.monitor.Name).FontSize(16).FontWeight(600).LetterSpacing(-0.2).SingleLine()
			ui.Text(c, p.caption()).FontSize(12).FontWeight(400).TextColor(muted).SingleLine()
		})
		more := ui.Box(c).Size(32, 32).Radius(16).Center().Cursor(ui.CursorPointer).Label(tr("更多", "More")).Tooltip(tr("更多", "More"))
		more.Background(ui.RGBA(255, 255, 255, 0.06)).Border(0.7, cardStroke).TextColor(muted)
		if more.Pressed() {
			more.Background(ui.RGBA(255, 255, 255, 0.16)).TextColor(ink)
		} else if more.Hovered() {
			more.Background(ui.RGBA(255, 255, 255, 0.11)).TextColor(ink)
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
		power.Background(ui.RGBA(255, 255, 255, 0.06)).Border(0.7, cardStroke).TextColor(muted)
		if power.Pressed() {
			power.Background(ui.RGBA(255, 255, 255, 0.16)).TextColor(ink)
		} else if power.Hovered() {
			power.Background(ui.RGBA(255, 255, 255, 0.11)).TextColor(ink)
		}
		power.Children(func() { icon(c, "power").FontSize(14) })
		if power.Clicked() {
			s.sleepDisplay()
		}
	})
}

// caption sums the monitor up: "外接显示器 · HDMI · 4K · 160Hz".
func (p *panel) caption() string {
	if p.s.captionStr != "" {
		return p.s.captionStr
	}
	return p.s.computeCaption()
}

// content scrolls the panel when it is taller than the screen allows.
func (p *panel) content(c *ui.Context) {
	s := p.s
	scroll := ui.Scroll(c.Key("content")).Height(min(max(p.contentHeight, 1), p.maxContent))
	scroll.Children(func() {
		inner := ui.Column(c).Gap(10).Padding(0, 0, 4, 0)
		inner.Children(func() {
			p.overview(c)
			if s.notice != "" {
				banner := ui.Row(c).Gap(7).Padding(8, 10).Radius(9).AlignItems(ui.Center).
					Background(ui.RGBA(255, 255, 255, 0.055)).Border(0.7, cardStroke).
					Cursor(ui.CursorPointer)
				banner.Children(func() {
					ui.Box(c).Size(5, 5).Radius(2.5).Background(accent)
					ui.Text(c, s.notice).FontSize(11).TextColor(ink).Grow(1)
				})
				if banner.Clicked() {
					s.notice = ""
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
func (p *panel) footer(c *ui.Context) {
	s := p.s
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
		ui.Box(c).Height(12)
		return
	}
	dot := ui.RGB(52, 199, 89)
	if s.link == nil && s.ddc == nil {
		dot = ui.RGB(255, 204, 0)
	}
	ui.Row(c).Gap(6).Padding(10, 0, 4, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		ui.Text(c, text).FontSize(11).FontWeight(430).TextColor(muted).Grow(1)
		if s.detectingAny() {
			ui.Spinner(c).Size(12, 12)
		}
	})
}
