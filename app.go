package main

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"monitorbar/internal/display"
)

// app is the menu bar icon and the panel it opens.
type app struct {
	s        *store
	panel    *panel
	tray     *mygo.Tray
	flyout   *mygo.Window // the panel window, created on first open and hidden on close
	quitting bool
}

func (a *app) ready(show bool) {
	useSystemLanguage(mygo.App.Locale())
	nameScenes()
	a.s = newStore()
	a.s.redraw = func() {
		if a.flyout != nil {
			a.flyout.Invalidate()
		}
	}
	a.panel = &panel{s: a.s, height: 600, maxContent: 640, onHeight: a.resize}

	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), IconIsTemplate: true, ToolTip: "MonitorBar"})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: tr("打开 MonitorBar", "Open MonitorBar"), Click: func(*mygo.MenuItem, *mygo.Window) { a.open() }},
		mygo.Separator(),
		{Label: tr("退出 MonitorBar", "Quit MonitorBar"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	a.tray.OnClick(a.toggle)
	a.tray.OnRightClick(func() {
		a.close()
		a.tray.SetMenu(menu) // for this popup only, so clicks keep toggling the panel
		defer a.tray.SetMenu(nil)
		a.tray.PopUpMenu()
	})

	changed := func() {
		a.s.displaysChanged()
		a.s.redraw()
	}
	// Launching the app again (Finder, Dock, or Spotlight) opens its panel.
	mygo.App.OnSecondInstance(func([]string, string) { a.open() })
	mygo.Screen.OnDisplaysChanged(changed)
	mygo.Power.OnResume(changed)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		a.s.shutdown()
		a.tray.Destroy()
	})

	a.s.detect()
	// Open the panel on the first launch (or with --show) so new users find the icon.
	if show || !a.s.prefs.LaunchedBefore {
		a.s.prefs.LaunchedBefore, a.s.dirty = true, true
		time.AfterFunc(300*time.Millisecond, func() { mygo.RunOnMain(a.open) })
	}
}

func (a *app) toggle() {
	if a.flyout != nil && a.flyout.IsVisible() {
		a.close()
	} else {
		a.open()
	}
}

func (a *app) open() {
	if a.flyout != nil && a.flyout.IsVisible() {
		return
	}
	a.s.panelOpened()
	icon := a.tray.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: icon.X + icon.Width/2, Y: icon.Y + icon.Height/2}).WorkArea
	a.panel.maxHeight = area.Height
	a.panel.updateMaxContent()
	h := min(a.panel.height, area.Height)
	if a.flyout != nil {
		a.flyout.SetSize(panelWidth, h)
		a.pointArrow()
		a.flyout.Show()
		return
	}
	a.flyout = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      a.tray,
		Placement: mygo.PlacementBottom,
		Width:     panelWidth,
		Height:    h,
		Focusable: false,
		Shadow:    true,
		Content:   ui.View(a.panel.view),
	})
	display.WatchOutsideClicks(func() {
		mygo.RunOnMain(a.close)
	})
	a.flyout.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.close()
		}
	})
	a.flyout.OnClosed(func() {
		a.flyout = nil
		a.s.save()
	})
	a.pointArrow()
}

func (a *app) close() {
	if a.flyout != nil && a.flyout.IsVisible() {
		a.flyout.Hide()
		a.s.save()
	}
}

// resize fits the panel to its content, which changes between states.
func (a *app) resize(height int) {
	if a.flyout == nil {
		return
	}
	if a.panel.maxHeight > 0 && height > a.panel.maxHeight {
		height = a.panel.maxHeight
	}
	a.flyout.SetSize(panelWidth, height)
	a.pointArrow()
}

// pointArrow aims the arrow at the icon's middle, wherever the panel was
// placed against the screen's edges.
func (a *app) pointArrow() {
	icon, win := a.tray.Bounds(), a.flyout.Bounds()
	a.panel.arrowX = float32(icon.X + icon.Width/2 - win.X)
	a.flyout.Invalidate()
}
