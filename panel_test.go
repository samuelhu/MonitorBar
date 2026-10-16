package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
	"github.com/binaryify/MonitorBar/internal/mitv"
)

// testPanel is the panel over a store of fixed data, connected to the
// display over the LAN, without touching the system.
func testPanel(t *testing.T) (*panel, *[2]float64) {
	t.Helper()
	var gamma [2]float64
	s := &store{
		prefs:    &settings{SoftBrightness: 100},
		redraw:   func() {},
		setGamma: func(b, w float64) error { gamma = [2]float64{b, w}; return nil },
		monitor:  display.Monitor{Name: "Mi Monitor", Connection: "HDMI"},
		link:     mitv.New("192.0.2.1"),
		inputs:   []mitv.Source{{ID: 23, Name: "HDMI1"}, {ID: 24, Name: "HDMI2"}, {ID: 29, Name: "DP"}, {ID: 30, Name: "USB-C"}},
		volume:   80, hasVolume: true, deviceVolume: 80,
	}
	for _, rate := range []int{160, 144, 120, 60} {
		for _, w := range []int{3360, 3200, 2560, 1920, 1600, 1280} {
			s.modes = append(s.modes, display.Mode{Width: w, Height: w * 9 / 16, PixelWidth: 2 * w, PixelHeight: w * 9 / 8, RefreshRate: rate})
		}
		for _, w := range []int{3840, 2560, 1920} {
			s.modes = append(s.modes, display.Mode{Width: w, Height: w * 9 / 16, PixelWidth: w, PixelHeight: w * 9 / 16, RefreshRate: rate})
		}
	}
	s.current, s.hasCurrent = s.modes[3], true // 1920 × 1080 HiDPI at 160 Hz
	s.updateDisplayCache()
	return &panel{s: s, maxContent: 850, arrowX: panelWidth / 2}, &gamma
}

func newTester(p *panel) *ui.Tester {
	tt := ui.NewTester(p.view, panelWidth, 1100)
	for range 3 { // the scroll area takes its content's height a frame later
		tt.Frame()
	}
	return tt
}

func TestOverview(t *testing.T) {
	p, _ := testPanel(t)
	tt := newTester(p)
	for _, want := range []string{"Mi Monitor", "HDMI · 1920 × 1080 @ 160Hz", "HDMI 1", "DP", "USB-C", "显示器音量", "已通过局域网连接 · 澎湃 OS 遥控"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("硬件控制未就绪") {
		t.Error("the notice shows while connected")
	}
}

func TestSceneSetsGamma(t *testing.T) {
	p, gamma := testPanel(t)
	tt := newTester(p)
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.s.prefs.SoftBrightness != 50 || p.s.prefs.Warmth != 25 || *gamma != [2]float64{0.5, 0.25} {
		t.Errorf("brightness %v warmth %v gamma %v", p.s.prefs.SoftBrightness, p.s.prefs.Warmth, *gamma)
	}
}

func TestUnifiedControls(t *testing.T) {
	p, _ := testPanel(t)
	tt := newTester(p)
	for _, w := range []string{
		"画面", "亮度", "冷暖", "显示器音量", "音量",
		"编程", "阅读", "影音",
		"刷新率", "160 Hz",
	} {
		if !tt.HasText(w) {
			t.Errorf("missing %q in %q", w, tt.Texts())
		}
	}
}

// With no channel to the monitor, the panel shows what works and says
// nothing about what doesn't.
func TestNoChannelShowsNoUnsupportedText(t *testing.T) {
	p, _ := testPanel(t)
	p.s.link, p.s.inputs, p.s.hasVolume = nil, nil, false
	tt := newTester(p)
	for _, text := range tt.Texts() {
		for _, word := range []string{"不支持", "未找到", "未就绪", "需要显示器"} {
			if strings.Contains(text, word) {
				t.Errorf("panel shows %q", text)
			}
		}
	}
	if tt.HasText("输入源") || tt.HasText("显示器音量") || tt.HasText("已通过局域网连接 · 澎湃 OS 遥控") {
		t.Errorf("panel shows controls of a missing channel: %q", tt.Texts())
	}
	if !tt.HasText("亮度") {
		t.Error("brightness control is missing")
	}
}

// A monitor reached over DDC/CI alone gets its inputs, its backlight and
// the picture controls it answers.
func TestDDCOnly(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	s.link, s.inputs = nil, nil
	s.hasBrightness, s.brightness = true, 70
	s.ddcInputs = []mitv.Source{{ID: 0x0F, Name: display.InputName(0x0F)}, {ID: 0x11, Name: display.InputName(0x11)}}
	s.controls = []control{{code: display.VCPContrast, name: "对比度", value: 50, max: 100}}
	tt := newTester(p)
	for _, want := range []string{"输入源", "DDC/CI", "DP", "HDMI 1", "亮度", "70", "对比度", "显示器"} {
		if !tt.HasText(want) {
			t.Errorf("overview: missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("阅读"); err != nil {
		t.Fatal(err)
	}
	if s.brightness != 50 || s.prefs.SoftBrightness != 100 {
		t.Errorf("reading scene: backlight %v, software %v; want the backlight dimmed", s.brightness, s.prefs.SoftBrightness)
	}
}

// TestSnapshots writes the panel as a PNG to $MONITORBAR_SNAPSHOTS, to look at.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("MONITORBAR_SNAPSHOTS")
	if dir == "" {
		t.Skip("set MONITORBAR_SNAPSHOTS to a directory")
	}
	p, _ := testPanel(t)
	tt := newTester(p)
	tt.SetScale(2)
	tt.Frame()
	img := tt.Image()
	crop := img.SubImage(image.Rect(0, 0, img.Bounds().Dx(), p.height*2))
	f, err := os.Create(filepath.Join(dir, "panel.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, crop); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := tt.Click("静音"); err == nil {
		tt.Frame()
		img = tt.Image()
		crop = img.SubImage(image.Rect(0, 0, img.Bounds().Dx(), p.height*2))
		if f, err := os.Create(filepath.Join(dir, "panel_muted.png")); err == nil {
			_ = png.Encode(f, crop)
			f.Close()
		}
	}

	// Capture user's exact state: Brightness 50%, Warmth 25%, Volume 19%
	p.s.hasBrightness = false
	p.s.prefs.SoftBrightness = 50
	p.s.prefs.Warmth = 25
	p.s.volume = 19
	p.s.deviceVolume = 19
	tt.Frame()
	img = tt.Image()
	crop = img.SubImage(image.Rect(0, 0, img.Bounds().Dx(), p.height*2))
	if f, err := os.Create(filepath.Join(dir, "panel_user_state.png")); err == nil {
		_ = png.Encode(f, crop)
		f.Close()
	}
}

// The panel must report its natural height even while its window is
// shorter, or it could never grow to fit a notice.
func TestPanelReportsNaturalHeight(t *testing.T) {
	p, _ := testPanel(t)
	tt := ui.NewTester(p.view, panelWidth, 600)
	for range 6 {
		tt.Frame()
	}
	natural := p.height
	t.Logf("natural height in a 600 DIP window: %d", natural)
	if natural <= 600 {
		t.Fatalf("height %d is clamped to the window", natural)
	}
	p.s.notice = "一条提示，让内容变高。"
	for range 6 {
		tt.Frame()
	}
	if p.height <= natural {
		t.Fatalf("height %d did not grow past %d with a notice", p.height, natural)
	}
}

// A notice that appears after a click must grow the panel by itself, with
// no other frame asked for: windows draw only on demand.
func TestNoticeGrowsPanelWithoutInput(t *testing.T) {
	p, _ := testPanel(t)
	p.s.setGamma = func(b, w float64) error {
		if b <= 0.5 {
			return display.ErrGammaLimited
		}
		return nil
	}
	tt := newTester(p)
	before := p.height
	if err := tt.Click("阅读"); err != nil { // Click draws only the frames the view asks for
		t.Fatal(err)
	}
	if !tt.HasText(gammaLimited()) {
		t.Fatalf("no notice in %q", tt.Texts())
	}
	if p.height <= before {
		t.Fatalf("height stayed %d after the notice appeared", p.height)
	}
}

// Mute is volume 0: the button drops the level to zero and restores it after.
func TestMuteIsVolumeZero(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	s.volume = 40
	tt := newTester(p)
	if err := tt.Click("静音"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.muted || s.volume != 0 {
		t.Fatalf("after mute: muted %v volume %v", s.muted, s.volume)
	}
	if !tt.HasText("已静音") {
		t.Errorf("no muted row in %q", tt.Texts())
	}
	// The muted row has a button that restores the previous level.
	if err := tt.Click("恢复音量"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.muted || s.volume != 40 {
		t.Fatalf("after unmute: muted %v volume %v, want 40", s.muted, s.volume)
	}
}

// The panel follows the system language: English locale shows English text.
func TestEnglishFollowsSystemLanguage(t *testing.T) {
	prev := lang
	t.Cleanup(func() { lang = prev; nameScenes() })
	useSystemLanguage("en-US")
	p, _ := testPanel(t)
	tt := newTester(p)
	for _, want := range []string{
		"Coding", "Reading", "Movie", "Picture", "Brightness", "Warmth",
		"Monitor volume", "Input", "Refresh rate",
		"Connected over LAN · HyperOS remote",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing English %q in %q", want, tt.Texts())
		}
	}
	for _, bad := range []string{"画面", "亮度", "显示器音量", "编程", "已通过局域网连接"} {
		if tt.HasText(bad) {
			t.Errorf("Chinese %q still shown in English mode", bad)
		}
	}
	if err := tt.Click("Mute"); err != nil {
		t.Fatalf("clicking Mute in English: %v", err)
	}
	tt.Frame()
	if !p.s.muted || !tt.HasText("Muted") {
		t.Errorf("after English mute: muted %v texts %q", p.s.muted, tt.Texts())
	}
	if err := tt.Click("Restore volume"); err != nil {
		t.Fatalf("clicking Restore volume in English: %v", err)
	}
	tt.Frame()
	if p.s.muted {
		t.Errorf("after English unmute: still muted")
	}
}

func TestUseSystemLanguage(t *testing.T) {
	prev := lang
	t.Cleanup(func() { lang = prev; nameScenes() })
	for _, c := range []struct{ locale, want string }{
		{"zh-Hans", "zh"}, {"zh-CN", "zh"}, {"en-US", "en"}, {"en", "en"}, {"fr-FR", "en"},
	} {
		useSystemLanguage(c.locale)
		if lang != c.want {
			t.Errorf("locale %q -> lang %q, want %q", c.locale, lang, c.want)
		}
	}
}

// TestSnapshotsEN writes the English panel when $MONITORBAR_SNAPSHOTS is set.
func TestSnapshotsEN(t *testing.T) {
	dir := os.Getenv("MONITORBAR_SNAPSHOTS")
	if dir == "" {
		t.Skip("set MONITORBAR_SNAPSHOTS to a directory")
	}
	prev := lang
	t.Cleanup(func() { lang = prev; nameScenes() })
	useSystemLanguage("en-US")
	p, _ := testPanel(t)
	tt := newTester(p)
	tt.SetScale(2)
	tt.Frame()
	img := tt.Image()
	crop := img.SubImage(image.Rect(0, 0, img.Bounds().Dx(), p.height*2))
	f, err := os.Create(filepath.Join(dir, "panel_en.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, crop); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := tt.Click("Mute"); err == nil {
		tt.Frame()
		img = tt.Image()
		crop = img.SubImage(image.Rect(0, 0, img.Bounds().Dx(), p.height*2))
		if f, err := os.Create(filepath.Join(dir, "panel_en_muted.png")); err == nil {
			_ = png.Encode(f, crop)
			f.Close()
		}
	}
}

// Clicking a LAN input records LastInput and sends the switch command, while
// clearing inputVerified so no fake active highlight is shown without DDC readback.
func TestInputSelectionClearsUnverifiedHighlight(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	s.inputVerified = true // e.g. from an earlier DDC probe
	if s.prefs.LastInput != 0 {
		t.Fatalf("fresh store already has LastInput %d", s.prefs.LastInput)
	}
	tt := newTester(p)
	if err := tt.Click("DP"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.input != 29 || s.prefs.LastInput != 29 {
		t.Fatalf("after clicking DP: input %d LastInput %d, want 29", s.input, s.prefs.LastInput)
	}
	if s.inputVerified {
		t.Fatalf("expected inputVerified to be cleared on LAN input click")
	}
}

func TestRefreshRatesPreserveMaxAndDedupe(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s

	// 1. A 75Hz office monitor (75, 60): when active is 60, 75Hz must not disappear
	s.modes = []display.Mode{
		{Width: 1920, Height: 1080, PixelWidth: 1920, PixelHeight: 1080, RefreshRate: 75},
		{Width: 1920, Height: 1080, PixelWidth: 1920, PixelHeight: 1080, RefreshRate: 60},
		// Duplicate 60Hz mode (e.g. RGB vs YCbCr)
		{Width: 1920, Height: 1080, PixelWidth: 1920, PixelHeight: 1080, RefreshRate: 60},
	}
	s.current, s.hasCurrent = s.modes[1], true // current is 60Hz
	s.updateDisplayCache()

	if len(s.cachedShowRates) != 2 {
		t.Fatalf("expected 2 unique rates for [75, 60], got %d: %+v", len(s.cachedShowRates), s.cachedShowRates)
	}
	if s.cachedShowRates[0].RefreshRate != 75 || s.cachedShowRates[1].RefreshRate != 60 {
		t.Fatalf("expected [75, 60], got [%d, %d]", s.cachedShowRates[0].RefreshRate, s.cachedShowRates[1].RefreshRate)
	}

	// 2. A 165Hz gaming monitor: when active is 60, 165Hz max rate must be preserved
	s.modes = []display.Mode{
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 165},
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 144},
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 120},
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 100},
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 60},
	}
	s.current, s.hasCurrent = s.modes[4], true // active is 60Hz
	s.updateDisplayCache()

	has165, has60 := false, false
	for _, m := range s.cachedShowRates {
		if m.RefreshRate == 165 {
			has165 = true
		}
		if m.RefreshRate == 60 {
			has60 = true
		}
	}
	if !has165 || !has60 {
		t.Fatalf("expected max rate 165Hz and active rate 60Hz in cachedShowRates, got %+v", s.cachedShowRates)
	}
}

func TestLastInputRestoredForDDC(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	s.link = nil // DDC only
	s.prefs.LastInput = 17

	s.restoreLastInput([]mitv.Source{{ID: 15, Name: "VGA"}, {ID: 17, Name: "HDMI1"}, {ID: 18, Name: "HDMI2"}})
	if s.input != 17 {
		t.Fatalf("expected s.input to be restored to 17, got %d", s.input)
	}

	// Stale / invalid input ID should not be set
	s.input = 0
	s.prefs.LastInput = 99 // not in sources
	s.restoreLastInput([]mitv.Source{{ID: 15, Name: "VGA"}, {ID: 17, Name: "HDMI1"}})
	if s.input != 0 {
		t.Fatalf("expected invalid LastInput 99 to be ignored, got %d", s.input)
	}
}

func TestLANInputNotRestoredAsFakeActive(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	s.prefs.LastInput = 24 // previously clicked HDMI 2
	s.restoreLastInput(s.inputs)
	if s.input != 0 {
		t.Fatalf("LAN mode should not restore unverified LastInput as active, got %d", s.input)
	}
}

func TestHostPortDefaultAndManualDesignation(t *testing.T) {
	p, _ := testPanel(t)
	s := p.s
	// testPanel has two HDMI ports (HDMI1=23, HDMI2=24) on an "HDMI" connection:
	// without manual designation, hostInput() returns 0 rather than guessing HDMI1.
	if got := s.hostInput(); got != 0 {
		t.Fatalf("expected ambiguous dual-HDMI hostInput() to return 0 until designated, got %d", got)
	}
	if !strings.Contains(s.inputNote(), "HDMI") || strings.Contains(s.inputNote(), "HDMI 1") {
		t.Fatalf("expected inputNote to show generic HDMI family without guessing HDMI 1, got %q", s.inputNote())
	}

	// "DisplayPort" connection matches single DP port (29) unambiguously
	s.monitor.Connection = "DisplayPort"
	if got := s.hostInput(); got != 29 {
		t.Fatalf("expected DisplayPort connection to match DP (29), got %d", got)
	}

	// "USB-C" connection matches single USB-C port (30) unambiguously
	s.monitor.Connection = "USB-C"
	if got := s.hostInput(); got != 30 {
		t.Fatalf("expected USB-C connection to match USB-C (30), got %d", got)
	}

	// Manually designating HDMI 2 (ID 24) on an HDMI connection resolves the ambiguity
	s.monitor.Connection = "HDMI"
	s.setHostInput(24)
	if s.prefs.HostInput != 24 {
		t.Fatalf("expected prefs.HostInput to be 24, got %d", s.prefs.HostInput)
	}
	if got := s.hostInput(); got != 24 {
		t.Fatalf("expected hostInput() to return 24, got %d", got)
	}
	if !strings.Contains(s.inputNote(), "HDMI 2") {
		t.Fatalf("expected inputNote to contain HDMI 2, got %q", s.inputNote())
	}
}

