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
	"time"

	"github.com/egoist/mygo"

	"monitorbar/internal/display"
	"monitorbar/internal/mitv"
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
const discoverCooldown = 3 * time.Minute

type store struct {
	prefs        *settings
	redraw       func()                                 // asks the open panel for a new frame
	setGamma     func(brightness, warmth float64) error // the monitor's, replaced in tests
	restoreGamma func()                                 // display.RestoreGamma, replaced in tests

	monitor    display.Monitor
	modes      []display.Mode
	current    display.Mode
	hasCurrent bool

	captionStr      string
	cachedShowRates []display.Mode // the tiers the panel lists (handle-free copies)

	link          *mitv.Link // the display's LAN remote, nil until found
	detecting     bool
	lastDiscover  time.Time
	inputs        []mitv.Source
	input         int  // the input chosen in the app, 0 before any
	inputVerified bool // true only when read directly from hardware (DDC/CI VCP 0x60)
	volume        float64
	hasVolume     bool
	muted         bool // volume 0 as mute; returnTo is the level to restore
	returnTo      float64

	launchAtLogin       bool
	notice              string
	noticeGen           uint64
	noticeTimer         *time.Timer
	noticeDelay         time.Duration
	dirty               bool // prefs changed since the last save
	savedSoftBrightness float64
	savedWarmth         float64
	savedInit           bool
	gammaTimer          *time.Timer
	closed              bool

	volumeTarget    atomic.Int64
	deviceVolume    int // the display's volume in hardware steps as last known
	deviceMaxVolume int // the display's max volume step (default 100 when 0)
	chasing         bool

	// DDC/CI, when the monitor answers it: hardware brightness, inputs and
	// volume (when the LAN remote has none), and picture controls.
	ddc            *display.DDC
	probingDDC     bool
	refreshingDDC  bool
	lastDDCRefresh time.Time
	brightness     float64 // hardware %, meaningful when hasBrightness
	hasBrightness  bool
	ddcInputs      []mitv.Source
	ddcVolume      bool
	controls       []control
	maxima         map[byte]int // the range of each VCP code
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
	prefs := loadSettings()
	s := &store{
		prefs:               prefs,
		redraw:              func() {},
		restoreGamma:        display.RestoreGamma,
		launchAtLogin:       mygo.App.OpenAtLogin(),
		noticeDelay:         4 * time.Second,
		savedSoftBrightness: prefs.SoftBrightness,
		savedWarmth:         prefs.Warmth,
		savedInit:           true,
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

// setNotice displays a banner and schedules auto-dismissal using a generation
// counter so an older timer never clears a newer notice.
func (s *store) setNotice(msg string) {
	s.notice = msg
	s.noticeGen++
	if s.noticeTimer != nil {
		s.noticeTimer.Stop()
		s.noticeTimer = nil
	}
	if msg != "" && s.noticeDelay > 0 && !s.closed {
		gen := s.noticeGen
		s.noticeTimer = time.AfterFunc(s.noticeDelay, func() {
			s.post(func() {
				if !s.closed && s.noticeGen == gen {
					s.notice = ""
				}
			})
		})
	}
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

func gammaFailed() string {
	return tr("调光失败：系统拒绝了这个设置。", "Dimming failed: the system refused the setting.")
}

func (s *store) applyGamma() {
	if s.closed {
		return
	}
	if !s.savedInit || s.prefs.SoftBrightness != s.savedSoftBrightness || s.prefs.Warmth != s.savedWarmth {
		s.dirty = true
		s.savedSoftBrightness = s.prefs.SoftBrightness
		s.savedWarmth = s.prefs.Warmth
		s.savedInit = true
	}
	brightness := s.prefs.SoftBrightness / 100
	if s.hasBrightness {
		brightness = 1 // DDC/CI owns brightness; gamma only adjusts warmth.
	}
	err := s.setGamma(brightness, s.prefs.Warmth/100)
	switch {
	case err != nil:
		s.setNotice(gammaFailed())
	case s.notice == gammaFailed():
		s.setNotice("")
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

// hostInput returns the source ID corresponding to this host's physical cable
// when known unambiguously (manual designation, DDC-verified active input, or
// a single port matching the OS-reported cable type).
func (s *store) hostInput() int {
	sources := s.sources()
	if s.prefs != nil && s.prefs.HostInput != 0 {
		for _, src := range sources {
			if src.ID == s.prefs.HostInput {
				return s.prefs.HostInput
			}
		}
	}
	if s.inputVerified && s.input != 0 {
		for _, src := range sources {
			if src.ID == s.input {
				return s.input
			}
		}
	}
	conn := strings.ToUpper(strings.TrimSpace(s.monitor.Connection))
	if conn == "" {
		return 0
	}
	isHDMI := strings.Contains(conn, "HDMI")
	isDP := strings.Contains(conn, "DISPLAYPORT") || conn == "DP" || strings.HasPrefix(conn, "DP ")
	isUSBC := strings.Contains(conn, "USB-C") || strings.Contains(conn, "TYPE-C") || strings.Contains(conn, "THUNDERBOLT")
	var matches []int
	for _, src := range sources {
		upper := strings.ToUpper(strings.TrimSpace(src.Name))
		switch {
		case isHDMI && strings.HasPrefix(upper, "HDMI"):
			matches = append(matches, src.ID)
		case isDP && (strings.HasPrefix(upper, "DP") || strings.HasPrefix(upper, "DISPLAYPORT")):
			matches = append(matches, src.ID)
		case isUSBC && (strings.HasPrefix(upper, "USB-C") || strings.HasPrefix(upper, "TYPE-C")):
			matches = append(matches, src.ID)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return 0
}

// setHostInput manually designates which port is connected to this host.
func (s *store) setHostInput(id int) {
	if s.prefs == nil {
		return
	}
	s.prefs.HostInput, s.dirty = id, true
	s.save()
	var name string
	for _, src := range s.sources() {
		if src.ID == id {
			name = formatSourceName(src.Name)
			break
		}
	}
	if name == "" {
		name = strconv.Itoa(id)
	}
	s.setNotice(fmt.Sprintf(tr("已将 %s 设为本机接口", "Set %s as host connection"), name))
}

// inputNote pairs the host's physical cable with the control channel.
func (s *store) inputNote() string {
	return s.inputNoteFor(s.hostInput())
}

// inputNoteFor formats the input header note using a pre-resolved hostID so
// callers rendering the routing card do not evaluate hostInput() twice.
func (s *store) inputNoteFor(hostID int) string {
	channel := s.inputChannel()
	var hostPortName string
	for _, src := range s.sources() {
		if src.ID == hostID {
			hostPortName = formatSourceName(src.Name)
			break
		}
	}
	if hostPortName != "" {
		return fmt.Sprintf(tr("本机 %s · %s", "Host %s · %s"), hostPortName, channel)
	}
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

// connectWithScan probes the cached host first; if unreachable (or empty), it
// runs a full /24 Discover scan only when forceDiscover is true or the scan
// cooldown has elapsed.
func (s *store) connectWithScan(forceDiscover bool) {
	if s.detecting {
		return
	}
	cached := ""
	if s.prefs != nil {
		cached = s.prefs.NetworkHost
	}
	canDiscover := forceDiscover || s.lastDiscover.IsZero() || time.Since(s.lastDiscover) >= discoverCooldown
	if cached == "" && !canDiscover {
		return
	}
	s.detecting = true
	go func() {
		// probeLink validates a host end-to-end; a host that answers but cannot
		// list sources is treated as unusable so discovery can fall back.
		type lanProbe struct {
			link      *mitv.Link
			sources   []mitv.Source
			rawVol    int
			maxVol    int
			volumeErr error
		}
		probeLink := func(host string) lanProbe {
			if host == "" {
				return lanProbe{volumeErr: errors.New("mitv: no host")}
			}
			candidate := mitv.New(host)
			if _, err := candidate.DeviceNameTimeout(2 * time.Second); err != nil {
				return lanProbe{volumeErr: err}
			}
			sources, err := candidate.Sources()
			if err != nil || len(sources) == 0 {
				return lanProbe{volumeErr: err}
			}
			rawVol, maxVol, volumeErr := candidate.VolumeRaw()
			return lanProbe{link: candidate, sources: sources, rawVol: rawVol, maxVol: maxVol, volumeErr: volumeErr}
		}

		result := lanProbe{}
		if cached != "" {
			result = probeLink(cached)
		}
		didDiscover := false
		if result.link == nil && canDiscover {
			didDiscover = true
			if host, err := mitv.Discover(); err == nil {
				result = probeLink(host)
			}
		}
		s.post(func() {
			s.detecting = false
			if didDiscover {
				s.lastDiscover = time.Now()
			}
			s.link = result.link
			if result.link == nil {
				s.inputs, s.hasVolume = nil, s.ddcVolume // DDC/CI may still have volume
				return
			}
			// LAN remote has no current-source telemetry; never carry a DDC
			// inputVerified flag onto LAN source chips.
			s.inputVerified = false
			s.prefs.NetworkHost, s.dirty = result.link.Host, true
			s.save()
			slices.SortFunc(result.sources, func(a, b mitv.Source) int { return a.ID - b.ID })
			s.inputs = result.sources
			if result.volumeErr == nil && result.maxVol > 0 {
				s.deviceVolume, s.deviceMaxVolume = result.rawVol, result.maxVol
				pct := max(0, min((result.rawVol*100+result.maxVol/2)/result.maxVol, 100))
				s.volume, s.hasVolume = float64(pct), true
			}
		})
	}()
}

func (s *store) selectInput(src mitv.Source) {
	s.input = src.ID
	link := s.link
	if link == nil {
		s.prefs.LastInput, s.dirty = src.ID, true
		if s.ddc != nil {
			s.ddc.Set(display.VCPInput, src.ID)
		}
		return
	}
	// LAN switching is fire-and-forget; never show a fake verified highlight
	// or overwrite DDC VCP LastInput with LAN source IDs.
	s.inputVerified = false
	label := formatSourceName(src.Name)
	s.setNotice(tr("已向显示器发送切换指令：", "Switch command sent: ") + label)
	go func() {
		if err := link.Select(src.ID); err != nil {
			s.post(func() {
				s.setNotice(tr("切换到 ", "Switching to ") + label + tr(" 失败：显示器没有响应", " failed: the display did not answer"))
			})
		}
	}()
}

func (s *store) setVolume(percent float64) {
	percent = max(0, min(percent, 100))
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

func (s *store) targetVolumeSteps() (target, maxVol int) {
	maxVol = s.deviceMaxVolume
	if maxVol <= 0 {
		maxVol = 100
	}
	pct := max(0, min(int(s.volumeTarget.Load()), 100))
	target = (pct*maxVol + 50) / 100
	return max(0, min(target, maxVol)), maxVol
}

// chaseVolume steps toward the latest target in hardware steps: the API only has ±1 keys.
func (s *store) chaseVolume() {
	if s.link == nil || s.chasing {
		return
	}
	s.chasing = true
	_, maxVol := s.targetVolumeSteps()
	link, actual := s.link, max(0, min(s.deviceVolume, maxVol))
	targetStep := func() int {
		pct := max(0, min(int(s.volumeTarget.Load()), 100))
		return max(0, min((pct*maxVol+50)/100, maxVol))
	}
	go func() {
		failed := false
		for target := targetStep(); target != actual; target = targetStep() {
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
			target, _ := s.targetVolumeSteps()
			switch {
			case failed:
				s.setNotice(tr("调节音量失败：显示器没有响应", "Volume failed: the display did not answer"))
			case target != actual:
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
		rawVol, maxVol, err := link.VolumeRaw()
		if err != nil || maxVol <= 0 {
			return
		}
		s.post(func() {
			if s.link != link {
				return
			}
			s.deviceVolume, s.deviceMaxVolume, s.hasVolume = rawVol, maxVol, true
			if !s.chasing {
				pct := max(0, min((rawVol*100+maxVol/2)/maxVol, 100))
				s.volume = float64(pct)
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
			if d != nil {
				s.lastDDCRefresh = time.Now()
			} else {
				s.lastDDCRefresh = time.Time{}
			}
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

func (s *store) onHardwareBrightnessLost() {
	// Reset software brightness to 100% so the LUT only preserves warmth and
	// never stacks dimming on top of the monitor's retained hardware backlight.
	s.prefs.SoftBrightness = 100
	s.applyGamma()
	s.setNotice(tr("硬件背光控制已断开，已改用软件调光（100%）", "Hardware backlight unavailable; switched to software dimming (100%)"))
}

func (s *store) applyDDC(r ddcReading) {
	if s.ddc == nil {
		wasHardwareBrightness := s.hasBrightness
		s.hasBrightness, s.ddcVolume, s.ddcInputs, s.controls, s.inputVerified = false, false, nil, nil, false
		if s.link == nil {
			s.hasVolume = false // DDC may have been the only volume source
		}
		if wasHardwareBrightness {
			s.onHardwareBrightnessLost()
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
		s.inputVerified = true
		s.prefs.LastInput, s.dirty = s.input, true
	} else if s.link == nil && !r.refreshed {
		s.inputVerified = false
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
	if wasHardwareBrightness && !s.hasBrightness {
		s.onHardwareBrightnessLost()
	} else if !wasHardwareBrightness && s.hasBrightness {
		s.applyGamma()
	}
}

// refreshDDC rereads the values the monitor's own buttons may have changed.
func (s *store) refreshDDC() {
	d := s.ddc
	if d == nil || s.probingDDC || s.refreshingDDC {
		return
	}
	if !s.lastDDCRefresh.IsZero() && time.Since(s.lastDDCRefresh) < 5*time.Second {
		return
	}
	s.refreshingDDC = true
	go func() {
		r := readDDC(d, false)
		s.post(func() {
			s.refreshingDDC = false
			if s.ddc == d {
				s.lastDDCRefresh = time.Now()
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
	target := m
	for _, live := range s.modes {
		if live.ID() == m.ID() {
			target = live
			break
		}
	}
	if err := s.monitor.Apply(target); err != nil {
		s.setNotice(tr("切换失败，显示器拒绝了该模式", "Switch failed: the display refused the mode"))
	} else {
		s.setNotice(tr("已切换到 ", "Switched to ") + target.Title() + " · " + strconv.Itoa(target.RefreshRate) + " Hz")
	}
	s.refreshDisplay()
}

func (s *store) sleepDisplay() { _ = display.SleepNow() }

// MARK: Lifecycle

func (s *store) releaseModes() {
	for _, m := range s.modes {
		m.Release()
	}
	if s.hasCurrent {
		s.current.Release()
	}
	s.modes = nil
	s.cachedShowRates = nil
	s.current = display.Mode{}
	s.hasCurrent = false
}

func (s *store) refreshDisplay() {
	s.releaseModes()
	s.monitor = display.Target()
	s.modes = s.monitor.Modes()
	s.current, s.hasCurrent = s.monitor.CurrentMode()
	s.updateDisplayCache()
}

func (s *store) shutdown() {
	s.closed = true
	if s.gammaTimer != nil {
		s.gammaTimer.Stop()
		s.gammaTimer = nil
	}
	if s.noticeTimer != nil {
		s.noticeTimer.Stop()
		s.noticeTimer = nil
	}
	s.save()
	if s.restoreGamma != nil {
		s.restoreGamma()
	} else {
		display.RestoreGamma()
	}
	if s.ddc != nil {
		s.ddc.Close()
		s.ddc = nil
	}
	s.releaseModes()
}

func (s *store) restoreLastInput(sources []mitv.Source) {
	// Only restore on DDC/CI where hardware state is verifiable.
	// In LAN mode, avoid restoring stale last-clicked input as active state.
	if s.link != nil {
		return
	}
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
	rates := display.RefreshRates(s.modes, s.current)
	rate := 60
	if s.hasCurrent {
		rate = s.current.RefreshRate
	}

	// Deduplicate modes by refresh rate (one chip per frequency), stripping
	// native handles so cachedShowRates never aliases handles owned by s.modes.
	var unique []display.Mode
	seen := map[int]bool{}
	for _, m := range rates {
		if !seen[m.RefreshRate] {
			seen[m.RefreshRate] = true
			unique = append(unique, m.WithoutHandle())
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
	// 3. 60 Hz baseline compatibility tier when present
	// 4. Highest remaining standard tiers (240, 180, 165, 160, 144, 120, 60) or remaining modes
	// Capped at 4 chips for a neat single-row layout.
	maxRate := unique[0].RefreshRate
	picked := map[int]bool{maxRate: true, rate: true}
	if seen[60] {
		picked[60] = true
	}

	standardTiers := []int{240, 180, 165, 160, 144, 120, 60}
	for _, tier := range standardTiers {
		if len(picked) >= 4 {
			break
		}
		if seen[tier] {
			picked[tier] = true
		}
	}
	for _, m := range unique {
		if len(picked) >= 4 {
			break
		}
		picked[m.RefreshRate] = true
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

// displaysChanged follows plugging, mode changes and wake: macOS or ColorSync
// may have reset the hardware LUT, so invalidate the last-pushed cache and
// re-push the gamma table (re-capturing the base table if the display handle
// or LUT capacity changed).
func (s *store) displaysChanged() {
	if s.closed {
		return
	}
	previous := s.monitor
	s.refreshDisplay()
	display.ResetGammaBase()
	s.applyGamma()
	if s.monitor != previous {
		s.probeDDC()
	}
	// On macOS, ColorSync may reload the display ICC profile ~300-600ms after
	// wake or display reconfiguration, overwriting early gamma writes.
	if s.gammaTimer != nil {
		s.gammaTimer.Stop()
	}
	if s.prefs != nil && s.prefs.path != "" && !s.closed {
		s.gammaTimer = time.AfterFunc(600*time.Millisecond, func() {
			s.post(func() {
				if s.closed {
					return
				}
				display.ResetGammaBase()
				s.applyGamma()
			})
		})
	}
}

func (s *store) panelOpened() {
	// Probe the cached LAN host on every open, but rate-limit full /24 scans.
	if s.link == nil {
		s.connectWithScan(false)
	} else {
		s.readVolume()
	}
	s.refreshDDC()
}

// detect looks for every control channel again.
func (s *store) detect() {
	s.connectWithScan(true)
	s.probeDDC()
}

// detectingAny reports a channel still being looked for.
func (s *store) detectingAny() bool { return s.detecting || s.probingDDC }

func (s *store) toggleLaunchAtLogin() {
	if err := mygo.App.SetOpenAtLogin(!s.launchAtLogin); err != nil {
		s.setNotice(tr("登录项设置失败：", "Login item failed: ") + err.Error())
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
