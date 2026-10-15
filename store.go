package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"

	"github.com/binaryify/MonitorBar/internal/display"
	"github.com/binaryify/MonitorBar/internal/mitv"
)

// scene is a convenience starting point for brightness and warmth.
type scene struct {
	name, icon         string
	brightness, warmth float64
	keepBrightness     bool // true when a scene should only change warmth
}

// scene names are set at launch from the system language (see nameScenes).
var scenes = []scene{
	{"编程", "code", 65, 10, false},
	{"阅读", "book", 50, 25, false},
	{"影音", "film", 0, 0, true},
}

func nameScenes() {
	names := [3][2]string{
		{"编程", "Coding"},
		{"阅读", "Reading"},
		{"影音", "Movie"},
	}
	for i := range scenes {
		scenes[i].name = tr(names[i][0], names[i][1])
	}
}

// store holds what the panel shows and the actions that change it. Its
// fields are read and written on the main thread only: network work runs in
// goroutines and hands its results back through post.
type store struct {
	prefs    *settings
	redraw   func()                                 // asks the open panel for a new frame
	setGamma func(brightness, warmth float64) error // the monitor's, replaced in tests

	monitor    display.Monitor
	modes      []display.Mode
	current    display.Mode
	hasCurrent bool

	captionStr      string
	cachedRates     []display.Mode
	cachedShowRates []display.Mode // the tiers the panel lists

	link      *mitv.Link // the display's LAN remote, nil until found
	detecting bool
	inputs    []mitv.Source
	input     int // the input chosen in the app, 0 before any
	volume    float64
	hasVolume bool
	muted     bool // volume 0 as mute; returnTo is the level to restore
	returnTo  float64

	launchAtLogin bool
	notice        string
	dirty         bool // prefs changed since the last save

	volumeTarget atomic.Int64
	deviceVolume int // the display's volume as last known
	chasing      bool

	// DDC/CI, when the monitor answers it: hardware brightness, inputs and
	// volume (when the LAN remote has none), and picture controls.
	ddc           *display.DDC
	probingDDC    bool
	refreshingDDC bool
	brightness    float64 // hardware %, meaningful when hasBrightness
	hasBrightness bool
	ddcInputs     []mitv.Source
	ddcVolume     bool
	controls      []control
	maxima        map[byte]int // the range of each VCP code
}

// control is a picture setting the monitor offers over DDC/CI.
type control struct {
	code  byte
	name  string
	value float64
	max   int
}

// controlCatalog lists the settings shown when the monitor answers them.
var controlCatalog = []struct {
	code byte
	name string
}{
	{display.VCPContrast, "对比度"}, {display.VCPSharpness, "锐度"},
	{display.VCPRedGain, "红色增益"}, {display.VCPGreenGain, "绿色增益"}, {display.VCPBlueGain, "蓝色增益"},
}

func newStore() *store {
	s := &store{
		prefs:         loadSettings(),
		redraw:        func() {},
		launchAtLogin: mygo.App.OpenAtLogin(),
	}
	s.refreshDisplay()
	s.setGamma = func(b, w float64) error { return s.monitor.SetGamma(b, w) }
	s.applyGamma()
	return s
}

// post runs fn on the main thread, then redraws.
func (s *store) post(fn func()) {
	mygo.RunOnMain(func() {
		fn()
		s.redraw()
	})
}

// MARK: Picture (software, through the gamma table)

// apply sets a scene's brightness, on the backlight when DDC/CI reaches it,
// and its warmth.
func (s *store) apply(sc scene) {
	if !sc.keepBrightness {
		if s.hasBrightness {
			s.setBrightness(sc.brightness)
		} else {
			s.prefs.SoftBrightness = sc.brightness
		}
	}
	s.prefs.Warmth = sc.warmth
	s.applyGamma()
}

func (s *store) isActive(sc scene) bool {
	return (sc.keepBrightness || math.Abs(s.effectiveBrightness()-sc.brightness) < 1) && math.Abs(s.prefs.Warmth-sc.warmth) < 1
}

// effectiveBrightness is the backlight's when DDC/CI reaches it, else the
// software dimming's.
func (s *store) effectiveBrightness() float64 {
	if s.hasBrightness {
		return s.brightness
	}
	return s.prefs.SoftBrightness
}

func gammaLimited() string {
	return tr("Windows 限制了软件调光的幅度，已调到它允许的最暗。", "Windows limited the software dimming; it is at the darkest it allows.")
}
func gammaFailed() string {
	return tr("调光失败：系统拒绝了这个设置。", "Dimming failed: the system refused the setting.")
}

func (s *store) applyGamma() {
	s.dirty = true
	brightness := s.prefs.SoftBrightness / 100
	if s.hasBrightness {
		brightness = 1 // DDC/CI owns brightness; gamma only adjusts warmth.
	}
	err := s.setGamma(brightness, s.prefs.Warmth/100)
	switch {
	case errors.Is(err, display.ErrGammaLimited):
		s.notice = gammaLimited()
	case err != nil:
		s.notice = gammaFailed()
	case s.notice == gammaLimited() || s.notice == gammaFailed():
		s.notice = ""
	}
}

// MARK: Network (HyperOS)

// sources are the inputs to switch between: the LAN remote's, else DDC/CI's.
func (s *store) sources() []mitv.Source {
	if s.link != nil {
		return s.inputs
	}
	return s.ddcInputs
}

// inputChannel names how inputs are switched, for the inputs card.
func (s *store) inputChannel() string {
	if s.link != nil {
		return tr("局域网", "LAN")
	}
	return "DDC/CI"
}

// inputNote pairs the host's physical cable with the control channel.
func (s *store) inputNote() string {
	channel := s.inputChannel()
	if s.monitor.Connection != "" {
		return fmt.Sprintf(tr("本机 %s · %s", "Host %s · %s"), s.monitor.Connection, channel)
	}
	return channel
}

// pictureNote says where brightness is applied: hardware backlight or gamma.
func (s *store) pictureNote() string {
	if s.hasBrightness {
		return tr("硬件背光", "Hardware backlight")
	}
	return tr("软件调光", "Software dimming")
}

// connect finds the display on the LAN: the host found last time first,
// then a scan.
func (s *store) connect() {
	if s.detecting {
		return
	}
	s.detecting = true
	cached := s.prefs.NetworkHost
	go func() {
		link := mitv.New(cached)
		if _, err := link.DeviceName(); cached == "" || err != nil {
			link = nil
			if host, err := mitv.Discover(); err == nil {
				link = mitv.New(host)
			}
		}
		var sources []mitv.Source
		volume, volumeErr := 0, error(nil)
		if link != nil {
			var err error
			if sources, err = link.Sources(); err != nil || len(sources) == 0 {
				link = nil
			} else {
				volume, volumeErr = link.Volume()
			}
		}
		s.post(func() {
			s.detecting = false
			s.link = link
			if link == nil {
				s.inputs, s.hasVolume = nil, s.ddcVolume // DDC/CI may still have volume
				return
			}
			s.prefs.NetworkHost, s.dirty = link.Host, true
			s.save()
			slices.SortFunc(sources, func(a, b mitv.Source) int { return a.ID - b.ID })
			s.inputs = sources
			s.restoreLastInput(s.inputs)
			if volumeErr == nil {
				s.deviceVolume, s.volume, s.hasVolume = volume, float64(volume), true
			}
		})
	}()
}

func (s *store) selectInput(src mitv.Source) {
	s.input = src.ID
	s.prefs.LastInput, s.dirty = src.ID, true
	link := s.link
	if link == nil {
		if s.ddc != nil {
			s.ddc.Set(display.VCPInput, src.ID)
		}
		return
	}
	go func() {
		if err := link.Select(src.ID); err != nil {
			s.post(func() {
				s.notice = tr("切换到 ", "Switching to ") + src.Name + tr(" 失败：显示器没有响应", " failed: the display did not answer")
			})
		}
	}()
}

func (s *store) setVolume(percent float64) {
	if percent > 0 {
		s.muted = false
	}
	s.volume = percent
	if s.link == nil {
		s.setDDC(display.VCPVolume, percent)
		return
	}
	s.volumeTarget.Store(int64(math.Round(percent)))
	s.chaseVolume()
}

// toggleMute mutes by setting volume to 0, and restores the level it had.
func (s *store) toggleMute() {
	if s.muted {
		s.muted = false
		s.setVolume(max(s.returnTo, 1))
		return
	}
	s.returnTo = max(s.volume, 1)
	s.muted = true
	s.setVolume(0)
}

// chaseVolume steps toward the latest target: the API only has ±1 keys.
func (s *store) chaseVolume() {
	if s.link == nil || s.chasing {
		return
	}
	s.chasing = true
	link, actual := s.link, s.deviceVolume
	go func() {
		failed := false
		for target := int(s.volumeTarget.Load()); target != actual; target = int(s.volumeTarget.Load()) {
			key, step := "volumeup", 1
			if target < actual {
				key, step = "volumedown", -1
			}
			if link.Press(key) != nil {
				failed = true
				break
			}
			actual += step
		}
		s.post(func() {
			s.chasing = false
			s.deviceVolume = actual
			switch {
			case failed:
				s.notice = tr("调节音量失败：显示器没有响应", "Volume failed: the display did not answer")
			case int(s.volumeTarget.Load()) != actual:
				s.chaseVolume() // the slider moved while the last step was sent
			default:
				s.readVolume()
			}
		})
	}()
}

func (s *store) readVolume() {
	link := s.link
	if link == nil {
		return
	}
	go func() {
		v, err := link.Volume()
		if err != nil {
			return
		}
		s.post(func() {
			s.deviceVolume, s.hasVolume = v, true
			if !s.chasing {
				s.volume = float64(v)
			}
		})
	}()
}

// MARK: DDC/CI

// probeDDC looks for DDC/CI on the monitor, in the background. A monitor
// that doesn't answer simply gets no DDC controls.
func (s *store) probeDDC() {
	if s.probingDDC {
		return
	}
	s.probingDDC = true
	if old := s.ddc; old != nil {
		s.ddc = nil
		go old.Close()
	}
	m := s.monitor
	go func() {
		d, err := m.OpenDDC()
		var r ddcReading
		if err == nil {
			r = readDDC(d, true)
		}
		s.post(func() {
			if s.monitor != m {
				if d != nil {
					go d.Close()
				}
				s.probingDDC = false
				s.probeDDC()
				return
			}
			s.probingDDC = false
			s.ddc = d
			s.applyDDC(r)
		})
	}()
}

// ddcReading is what the monitor reported over DDC/CI.
type ddcReading struct {
	values    map[byte][2]int // value and maximum of each code that answered
	inputs    []mitv.Source
	refreshed bool // values only: inputs and controls were not read again
}

// readDDC reads the values the panel shows; with all, also the inputs the
// capability string lists and which picture controls answer. It blocks.
func readDDC(d *display.DDC, all bool) ddcReading {
	r := ddcReading{values: map[byte][2]int{}, refreshed: !all}
	codes := []byte{display.VCPBrightness, display.VCPVolume, display.VCPInput}
	if all {
		for _, c := range controlCatalog {
			codes = append(codes, c.code)
		}
		for _, id := range display.ParseCapabilities(d.Capabilities())[display.VCPInput] {
			r.inputs = append(r.inputs, mitv.Source{ID: id, Name: display.InputName(id)})
		}
	}
	for _, code := range codes {
		if v, max, ok := d.Get(code); ok && max > 0 {
			r.values[code] = [2]int{v, max}
		}
	}
	return r
}

func (s *store) applyDDC(r ddcReading) {
	if s.ddc == nil {
		wasHardwareBrightness := s.hasBrightness
		s.hasBrightness, s.ddcVolume, s.ddcInputs, s.controls = false, false, nil, nil
		if wasHardwareBrightness {
			s.applyGamma()
		}
		return
	}
	wasHardwareBrightness := s.hasBrightness
	if s.maxima == nil {
		s.maxima = map[byte]int{}
	}
	for code, v := range r.values {
		s.maxima[code] = v[1]
	}
	percent := func(code byte) (float64, bool) {
		v, ok := r.values[code]
		return float64(v[0]) * 100 / float64(v[1]), ok
	}
	if v, ok := percent(display.VCPBrightness); ok {
		s.brightness, s.hasBrightness = v, true
	} else if !r.refreshed {
		// A full probe for a different monitor may have replaced a previous
		// monitor that supported the brightness code.
		s.hasBrightness = false
	}
	if v, ok := percent(display.VCPVolume); ok {
		s.ddcVolume = true
		if s.link == nil && !s.chasing {
			s.volume, s.hasVolume = v, true
		}
	}
	if v, ok := r.values[display.VCPInput]; ok && s.link == nil {
		s.input = v[0] & 0xFF // some monitors put more in the high byte
		s.prefs.LastInput, s.dirty = s.input, true
	}
	if r.refreshed {
		for i := range s.controls {
			if v, ok := r.values[s.controls[i].code]; ok {
				s.controls[i].value = float64(v[0])
			}
		}
		return
	}
	s.ddcInputs = r.inputs
	s.restoreLastInput(s.ddcInputs)
	s.controls = nil
	for _, c := range controlCatalog {
		if v, ok := r.values[c.code]; ok {
			s.controls = append(s.controls, control{code: c.code, name: controlName(c.code, c.name), value: float64(v[0]), max: v[1]})
		}
	}
	if wasHardwareBrightness != s.hasBrightness {
		s.applyGamma()
	}
}

// refreshDDC rereads the values the monitor's own buttons may have changed.
func (s *store) refreshDDC() {
	d := s.ddc
	if d == nil || s.probingDDC || s.refreshingDDC {
		return
	}
	s.refreshingDDC = true
	go func() {
		r := readDDC(d, false)
		s.post(func() {
			s.refreshingDDC = false
			if s.ddc == d {
				s.applyDDC(r)
			}
		})
	}()
}

// setBrightness sets the backlight over DDC/CI.
func (s *store) setBrightness(percent float64) {
	s.brightness = percent
	s.setDDC(display.VCPBrightness, percent)
}

// setDDC writes a percentage scaled to the code's range.
func (s *store) setDDC(code byte, percent float64) {
	if s.ddc == nil {
		return
	}
	max := s.maxima[code]
	if max == 0 {
		max = 100
	}
	s.ddc.Set(code, int(math.Round(percent/100*float64(max))))
}

func (s *store) setControl(i int) {
	if s.ddc != nil {
		s.ddc.Set(s.controls[i].code, int(math.Round(s.controls[i].value)))
	}
}

// MARK: Display modes

func (s *store) applyMode(m display.Mode) {
	if err := s.monitor.Apply(m); err != nil {
		s.notice = tr("切换失败，显示器拒绝了该模式", "Switch failed: the display refused the mode")
	} else {
		s.notice = tr("已切换到 ", "Switched to ") + m.Title() + " · " + strconv.Itoa(m.RefreshRate) + " Hz"
	}
	s.refreshDisplay()
}

func (s *store) sleepDisplay() { _ = display.SleepNow() }

// MARK: Lifecycle

func (s *store) refreshDisplay() {
	for _, m := range s.modes {
		m.Release()
	}
	if s.hasCurrent {
		s.current.Release()
	}
	s.monitor = display.Target()
	s.modes = s.monitor.Modes()
	s.current, s.hasCurrent = s.monitor.CurrentMode()
	s.updateDisplayCache()
}

func (s *store) restoreLastInput(sources []mitv.Source) {
	if s.input == 0 && s.prefs.LastInput != 0 {
		for _, src := range sources {
			if src.ID == s.prefs.LastInput {
				s.input = s.prefs.LastInput
				break
			}
		}
	}
}

func (s *store) updateDisplayCache() {
	s.captionStr = s.computeCaption()
	s.cachedRates = display.RefreshRates(s.modes, s.current)
	rate := 60
	if s.hasCurrent {
		rate = s.current.RefreshRate
	}

	// Deduplicate modes by refresh rate (one chip per frequency).
	var unique []display.Mode
	seen := map[int]bool{}
	for _, m := range s.cachedRates {
		if !seen[m.RefreshRate] {
			seen[m.RefreshRate] = true
			unique = append(unique, m)
		}
	}

	// If 4 or fewer unique rates exist, show all of them without dropping any.
	if len(unique) <= 4 {
		s.cachedShowRates = unique
		return
	}

	// For monitors with > 4 rates, keep:
	// 1. The monitor's highest supported rate (unique[0], so maximum capability is never lost)
	// 2. The currently active rate (rate)
	// 3. Standard common desktop tiers (240, 180, 165, 160, 144, 120, 60)
	// Capped at 4 chips for a neat single-row layout.
	maxRate := unique[0].RefreshRate
	picked := map[int]bool{maxRate: true, rate: true}

	standardTiers := []int{240, 180, 165, 160, 144, 120, 60}
	for _, tier := range standardTiers {
		if len(picked) >= 4 {
			break
		}
		if seen[tier] {
			picked[tier] = true
		}
	}

	var show []display.Mode
	for _, m := range unique {
		if picked[m.RefreshRate] {
			show = append(show, m)
		}
	}
	if len(show) == 0 {
		show = unique
	}
	s.cachedShowRates = show
}

func (s *store) computeCaption() string {
	parts := []string{}
	if s.monitor.Connection != "" {
		parts = append(parts, s.monitor.Connection)
	}
	if s.hasCurrent {
		parts = append(parts, fmt.Sprintf("%s @ %dHz", s.current.Title(), s.current.RefreshRate))
	} else {
		if label := display.SizeLabel(s.monitor.NativeSize(s.modes, s.current)); label != "" {
			parts = append(parts, label)
		}
		fastest := 0
		for _, m := range s.modes {
			fastest = max(fastest, m.RefreshRate)
		}
		if fastest > 0 {
			parts = append(parts, strconv.Itoa(fastest)+"Hz")
		}
	}
	if len(parts) == 0 {
		return tr("外接显示器", "External display")
	}
	return strings.Join(parts, " · ")
}

// displaysChanged follows plugging, mode changes and wake: the gamma table
// was reset, so capture it again and reapply.
func (s *store) displaysChanged() {
	previous := s.monitor
	s.refreshDisplay()
	display.ResetGammaBase()
	s.applyGamma()
	if s.monitor != previous {
		s.probeDDC()
	}
}

func (s *store) panelOpened() {
	// Retry here too: the launch attempt may have raced the network
	// permission prompt.
	if s.link == nil {
		s.connect()
	} else {
		s.readVolume()
	}
	s.refreshDDC()
}

// detect looks for every control channel again.
func (s *store) detect() {
	s.connect()
	s.probeDDC()
}

// detectingAny reports a channel still being looked for.
func (s *store) detectingAny() bool { return s.detecting || s.probingDDC }

func (s *store) toggleLaunchAtLogin() {
	if err := mygo.App.SetOpenAtLogin(!s.launchAtLogin); err != nil {
		s.notice = tr("登录项设置失败：", "Login item failed: ") + err.Error()
	}
	s.launchAtLogin = mygo.App.OpenAtLogin()
}

func (s *store) save() {
	if s.dirty {
		if err := s.prefs.save(); err != nil {
			log.Printf("settings: %v", err)
			return
		}
		s.dirty = false
	}
}

// controlName localizes the picture-control labels.
func controlName(code byte, zh string) string {
	switch code {
	case display.VCPContrast:
		return tr("对比度", "Contrast")
	case display.VCPSharpness:
		return tr("锐度", "Sharpness")
	case display.VCPRedGain:
		return tr("红色增益", "Red gain")
	case display.VCPGreenGain:
		return tr("绿色增益", "Green gain")
	case display.VCPBlueGain:
		return tr("蓝色增益", "Blue gain")
	}
	return zh
}
