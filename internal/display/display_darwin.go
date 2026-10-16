//go:build darwin

package display

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// SettingsURL opens the Displays pane of System Settings.
const SettingsURL = "x-apple.systempreferences:com.apple.Displays-Settings.extension"

type (
	handle     = uint32  // CGDirectDisplayID
	nativeMode = uintptr // CGDisplayModeRef, retained
)

var (
	cgGetActiveDisplayList             func(max uint32, ids *uint32, count *uint32) int32
	cgMainDisplayID                    func() uint32
	cgDisplayIsBuiltin                 func(id uint32) uint32
	cgDisplayIsAsleep                  func(id uint32) uint32
	cgDisplayCopyAllDisplayModes       func(id uint32, options uintptr) uintptr
	cgDisplayCopyDisplayMode           func(id uint32) uintptr
	cgDisplayModeGetWidth              func(mode uintptr) uintptr
	cgDisplayModeGetHeight             func(mode uintptr) uintptr
	cgDisplayModeGetPixelWidth         func(mode uintptr) uintptr
	cgDisplayModeGetPixelHeight        func(mode uintptr) uintptr
	cgDisplayModeGetRefreshRate        func(mode uintptr) float64
	cgDisplayModeIsUsableForDesktopGUI func(mode uintptr) bool
	cgBeginDisplayConfiguration        func(config *uintptr) int32
	cgConfigureDisplayWithDisplayMode  func(config uintptr, id uint32, mode uintptr, options uintptr) int32
	cgCompleteDisplayConfiguration     func(config uintptr, option uint32) int32
	cgDisplayGammaTableCapacity        func(id uint32) uint32
	cgGetDisplayTransferByTable        func(id, capacity uint32, r, g, b *float32, count *uint32) int32
	cgSetDisplayTransferByTable        func(id, size uint32, r, g, b *float32) int32
	cgDisplayRestoreColorSyncSettings  func()
	cgEventSourceSecondsSinceLast      func(state int32, eventType uint32) float64

	cfArrayGetCount           func(array uintptr) int
	cfArrayGetValueAtIndex    func(array uintptr, i int) uintptr
	cfRetain                  func(ref uintptr) uintptr
	cfRelease                 func(ref uintptr)
	cfDictionaryCreate        func(alloc uintptr, keys, values *uintptr, n int, keyCallbacks, valueCallbacks uintptr) uintptr
	cfDictionaryGetValue      func(dict, key uintptr) uintptr
	cfStringCreateWithCString func(alloc uintptr, s string, encoding uint32) uintptr
	cfStringGetCString        func(s uintptr, buf *byte, size int, encoding uint32) bool
	cfGetTypeID               func(ref uintptr) uintptr
	cfStringGetTypeID         func() uintptr

	ioServiceMatching               func(name string) uintptr
	ioServiceGetMatchingServices    func(port uint32, matching uintptr, iter *uint32) int32
	ioIteratorNext                  func(iter uint32) uint32
	ioObjectRelease                 func(obj uint32) int32
	ioRegistryEntryCreateCFProperty func(entry uint32, key, alloc uintptr, options uint32) uintptr

	modeOptions uintptr // {kCGDisplayShowDuplicateLowResolutionModes: true}, for HiDPI modes
	loadOnce    sync.Once
)

const utf8Encoding = 0x08000100

func load() {
	loadOnce.Do(func() {
		lib := func(path string) uintptr {
			h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				panic(err)
			}
			return h
		}
		cg := lib("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		cf := lib("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		io := lib("/System/Library/Frameworks/IOKit.framework/IOKit")
		lib("/System/Library/Frameworks/AppKit.framework/AppKit")
		for fn, name := range map[any]string{
			&cgGetActiveDisplayList: "CGGetActiveDisplayList", &cgMainDisplayID: "CGMainDisplayID",
			&cgDisplayIsBuiltin: "CGDisplayIsBuiltin", &cgDisplayIsAsleep: "CGDisplayIsAsleep",
			&cgDisplayCopyAllDisplayModes: "CGDisplayCopyAllDisplayModes", &cgDisplayCopyDisplayMode: "CGDisplayCopyDisplayMode",
			&cgDisplayModeGetWidth: "CGDisplayModeGetWidth", &cgDisplayModeGetHeight: "CGDisplayModeGetHeight",
			&cgDisplayModeGetPixelWidth: "CGDisplayModeGetPixelWidth", &cgDisplayModeGetPixelHeight: "CGDisplayModeGetPixelHeight",
			&cgDisplayModeGetRefreshRate: "CGDisplayModeGetRefreshRate", &cgDisplayModeIsUsableForDesktopGUI: "CGDisplayModeIsUsableForDesktopGUI",
			&cgBeginDisplayConfiguration: "CGBeginDisplayConfiguration", &cgConfigureDisplayWithDisplayMode: "CGConfigureDisplayWithDisplayMode",
			&cgCompleteDisplayConfiguration: "CGCompleteDisplayConfiguration", &cgDisplayGammaTableCapacity: "CGDisplayGammaTableCapacity",
			&cgGetDisplayTransferByTable: "CGGetDisplayTransferByTable", &cgSetDisplayTransferByTable: "CGSetDisplayTransferByTable",
			&cgDisplayRestoreColorSyncSettings: "CGDisplayRestoreColorSyncSettings", &cgEventSourceSecondsSinceLast: "CGEventSourceSecondsSinceLastEventType",
		} {
			purego.RegisterLibFunc(fn, cg, name)
		}
		for fn, name := range map[any]string{
			&cfArrayGetCount: "CFArrayGetCount", &cfArrayGetValueAtIndex: "CFArrayGetValueAtIndex",
			&cfRetain: "CFRetain", &cfRelease: "CFRelease", &cfDictionaryCreate: "CFDictionaryCreate",
			&cfDictionaryGetValue: "CFDictionaryGetValue", &cfStringCreateWithCString: "CFStringCreateWithCString",
			&cfStringGetCString: "CFStringGetCString", &cfGetTypeID: "CFGetTypeID", &cfStringGetTypeID: "CFStringGetTypeID",
		} {
			purego.RegisterLibFunc(fn, cf, name)
		}
		for fn, name := range map[any]string{
			&ioServiceMatching: "IOServiceMatching", &ioServiceGetMatchingServices: "IOServiceGetMatchingServices",
			&ioIteratorNext: "IOIteratorNext", &ioObjectRelease: "IOObjectRelease",
			&ioRegistryEntryCreateCFProperty: "IORegistryEntryCreateCFProperty",
		} {
			purego.RegisterLibFunc(fn, io, name)
		}
		key := symbolValue(cg, "kCGDisplayShowDuplicateLowResolutionModes")
		yes := symbolValue(cf, "kCFBooleanTrue")
		keyCallbacks, _ := purego.Dlsym(cf, "kCFTypeDictionaryKeyCallBacks")
		valueCallbacks, _ := purego.Dlsym(cf, "kCFTypeDictionaryValueCallBacks")
		modeOptions = cfDictionaryCreate(0, &key, &yes, 1, keyCallbacks, valueCallbacks)
	})
}

// symbolValue reads a global pointer variable such as kCFBooleanTrue.
func symbolValue(lib uintptr, name string) uintptr {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(err)
	}
	return **(**uintptr)(unsafe.Pointer(&addr))
}

// Target returns the first external display, or the main one.
func Target() Monitor {
	load()
	var ids [16]uint32
	var n uint32
	cgGetActiveDisplayList(uint32(len(ids)), &ids[0], &n)
	id := cgMainDisplayID()
	for _, d := range ids[:n] {
		if cgDisplayIsBuiltin(d) == 0 {
			id = d
			break
		}
	}
	return Monitor{Name: screenName(id), Connection: connection(), handle: id}
}

var (
	selScreens           objc.SEL
	selDeviceDescription objc.SEL
	selObjectForKey      objc.SEL
	selUnsignedIntValue  objc.SEL
	selLocalizedName     objc.SEL
	selUTF8String        objc.SEL
	selCount             objc.SEL
	selObjectAtIndex     objc.SEL
	selStringWithUTF8    objc.SEL
	selInitOnce          sync.Once
)

func initSelectors() {
	selInitOnce.Do(func() {
		selScreens = objc.RegisterName("screens")
		selDeviceDescription = objc.RegisterName("deviceDescription")
		selObjectForKey = objc.RegisterName("objectForKey:")
		selUnsignedIntValue = objc.RegisterName("unsignedIntValue")
		selLocalizedName = objc.RegisterName("localizedName")
		selUTF8String = objc.RegisterName("UTF8String")
		selCount = objc.RegisterName("count")
		selObjectAtIndex = objc.RegisterName("objectAtIndex:")
		selStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
	})
}

// screenName asks NSScreen for the display's localized name.
func screenName(id uint32) string {
	initSelectors()
	screens := objc.ID(objc.GetClass("NSScreen")).Send(selScreens)
	key := objc.ID(objc.GetClass("NSString")).Send(selStringWithUTF8, "NSScreenNumber")
	for i := range objc.Send[uint64](screens, selCount) {
		screen := screens.Send(selObjectAtIndex, i)
		number := screen.Send(selDeviceDescription).Send(selObjectForKey, key)
		if objc.Send[uint32](number, selUnsignedIntValue) == id {
			return cString(objc.Send[*byte](screen.Send(selLocalizedName), selUTF8String))
		}
	}
	return "Display"
}

// connection reads the cable type ("HDMI", "DisplayPort") from IOKit.
func connection() string {
	var iter uint32
	if ioServiceGetMatchingServices(0, ioServiceMatching("IOPortTransportStateDisplayPort"), &iter) != 0 {
		return ""
	}
	defer ioObjectRelease(iter)
	metadata := cfStringCreateWithCString(0, "Metadata", utf8Encoding)
	defer cfRelease(metadata)
	field := cfStringCreateWithCString(0, "DFP Type Description", utf8Encoding)
	defer cfRelease(field)
	for entry := ioIteratorNext(iter); entry != 0; entry = ioIteratorNext(iter) {
		dict := ioRegistryEntryCreateCFProperty(entry, metadata, 0, 0)
		ioObjectRelease(entry)
		if dict == 0 {
			continue
		}
		value := goCFString(cfDictionaryGetValue(dict, field))
		cfRelease(dict)
		if value != "" {
			return value
		}
	}
	return ""
}

func (m Monitor) Modes() []Mode {
	load()
	array := cgDisplayCopyAllDisplayModes(m.handle, modeOptions)
	if array == 0 {
		return nil
	}
	defer cfRelease(array)
	var modes []Mode
	for i := range cfArrayGetCount(array) {
		ref := cfArrayGetValueAtIndex(array, i)
		if cgDisplayModeIsUsableForDesktopGUI(ref) {
			modes = append(modes, newMode(cfRetain(ref)))
		}
	}
	return dedupe(modes)
}

func (m Monitor) CurrentMode() (Mode, bool) {
	load()
	ref := cgDisplayCopyDisplayMode(m.handle)
	if ref == 0 {
		return Mode{}, false
	}
	defer cfRelease(ref)
	return newMode(ref).WithoutHandle(), true
}

func newMode(ref uintptr) Mode {
	if ref == 0 {
		return Mode{}
	}
	rate := int(cgDisplayModeGetRefreshRate(ref) + 0.5)
	if rate == 0 {
		rate = 60
	}
	w, h := int(cgDisplayModeGetWidth(ref)), int(cgDisplayModeGetHeight(ref))
	pw, ph := int(cgDisplayModeGetPixelWidth(ref)), int(cgDisplayModeGetPixelHeight(ref))
	sizeKey := fmt.Sprintf("%dx%d@%v", w, h, pw > w)
	return Mode{
		Width: w, Height: h,
		PixelWidth: pw, PixelHeight: ph,
		RefreshRate: rate,
		sizeKey:     sizeKey,
		id:          fmt.Sprintf("%s/%d", sizeKey, rate),
		native:      ref,
	}
}

// Release frees the retained native CGDisplayModeRef.
func (m Mode) Release() {
	if m.native != 0 {
		cfRelease(m.native)
	}
}

func (m Monitor) Apply(mode Mode) error {
	load()
	var config uintptr
	if cgBeginDisplayConfiguration(&config) != 0 {
		return errors.New("display: cannot begin configuration")
	}
	cgConfigureDisplayWithDisplayMode(config, m.handle, mode.native, 0)
	if cgCompleteDisplayConfiguration(config, 2 /* kCGConfigurePermanently */) != 0 {
		return errors.New("display: the display refused the mode")
	}
	return nil
}

var (
	gammaMu        sync.Mutex
	gammaMonitor   uint32       // display handle gammaBase was captured from
	gammaBase      [3][]float32 // the ColorSync curve, captured before the first change
	gammaBuf       [3][]float32 // reusable buffer to avoid allocations during slider dragging
	gammaApplied   bool         // true while a custom gamma table is active
	lastBrightness = -1.0
	lastWarmth     = -999.0
)

// SetGamma dims and warms the picture through the gamma table. macOS resets
// it when the app quits.
func (m Monitor) SetGamma(brightness, warmth float64) error {
	load()
	if neutral(brightness, warmth) {
		RestoreGamma()
		return nil
	}
	gammaMu.Lock()
	defer gammaMu.Unlock()

	capacity := cgDisplayGammaTableCapacity(m.handle)
	if capacity == 0 {
		return errors.New("display: gamma table unsupported")
	}

	// Skip redundant system calls if setting and LUT capacity haven't changed on the same monitor.
	if gammaBase[0] != nil && gammaMonitor == m.handle && int(capacity) == len(gammaBase[0]) && brightness == lastBrightness && warmth == lastWarmth {
		return nil
	}

	if gammaBase[0] == nil || gammaMonitor != m.handle || int(capacity) != len(gammaBase[0]) {
		if gammaApplied {
			cgDisplayRestoreColorSyncSettings()
			gammaApplied = false
		}
		r, g, b := make([]float32, capacity), make([]float32, capacity), make([]float32, capacity)
		var n uint32
		cgGetDisplayTransferByTable(m.handle, capacity, &r[0], &g[0], &b[0], &n)
		if n == 0 {
			return errors.New("display: empty gamma table")
		}
		gammaBase = [3][]float32{r[:n], g[:n], b[:n]}
		gammaMonitor = m.handle
	}
	if len(gammaBase[0]) == 0 {
		return errors.New("display: no gamma table")
	}

	n := len(gammaBase[0])
	if len(gammaBuf[0]) != n {
		gammaBuf = [3][]float32{
			make([]float32, n),
			make([]float32, n),
			make([]float32, n),
		}
	}

	gr, gg, gb := gains(brightness, warmth)
	gainsArr := [3]float32{float32(gr), float32(gg), float32(gb)}
	for i := 0; i < 3; i++ {
		gain := gainsArr[i]
		baseChan := gammaBase[i]
		bufChan := gammaBuf[i]
		for j := 0; j < n; j++ {
			bufChan[j] = baseChan[j] * gain
		}
	}

	if cgSetDisplayTransferByTable(m.handle, uint32(n), &gammaBuf[0][0], &gammaBuf[1][0], &gammaBuf[2][0]) != 0 {
		return errors.New("display: the gamma table was refused")
	}
	gammaApplied = true
	lastBrightness = brightness
	lastWarmth = warmth
	return nil
}

// RestoreGamma puts the ColorSync curves back.
func RestoreGamma() {
	load()
	gammaMu.Lock()
	wasApplied := gammaApplied
	gammaApplied = false
	lastBrightness, lastWarmth = 1.0, 0.0
	gammaMu.Unlock()
	if wasApplied {
		cgDisplayRestoreColorSyncSettings()
	}
}

// ResetGammaBase invalidates the last-pushed brightness/warmth cache so the
// next SetGamma call pushes the transfer table to hardware again without
// flashing to 100% via cgDisplayRestoreColorSyncSettings (re-capturing the
// base table only if the display handle or LUT capacity changed).
func ResetGammaBase() {
	gammaMu.Lock()
	lastBrightness = -1.0
	lastWarmth = -999.0
	gammaMu.Unlock()
}

func (m Monitor) Asleep() bool {
	load()
	return cgDisplayIsAsleep(m.handle) != 0
}

// IdleSeconds is the time since the last keyboard or mouse input.
func IdleSeconds() float64 {
	load()
	return cgEventSourceSecondsSinceLast(1 /* combined session */, 0xFFFFFFFF /* any input */)
}

// SleepNow turns the displays off until the next input.
func SleepNow() error { return exec.Command("/usr/bin/pmset", "displaysleepnow").Start() }

var (
	outsideWatchMu   sync.Mutex
	outsideClickFn   func()
	outsideWatchOnce sync.Once
	outsideMonitor   objc.ID
)

// WatchOutsideClicks calls fn whenever a mouse button is pressed outside the
// application on macOS, allowing a non-activating, non-key flyout to dismiss
// when clicking elsewhere without stealing key-window focus from the active app.
func WatchOutsideClicks(fn func()) {
	load()
	outsideWatchMu.Lock()
	outsideClickFn = fn
	outsideWatchMu.Unlock()
	outsideWatchOnce.Do(func() {
		const mask = 1<<1 | 1<<3 | 1<<25 // NSEventMaskLeftMouseDown | RightMouseDown | OtherMouseDown
		selAdd := objc.RegisterName("addGlobalMonitorForEventsMatchingMask:handler:")
		selRetain := objc.RegisterName("retain")
		blk := objc.NewBlock(func(_ objc.Block, _ objc.ID) {
			outsideWatchMu.Lock()
			cb := outsideClickFn
			outsideWatchMu.Unlock()
			if cb != nil {
				cb()
			}
		})
		mon := objc.ID(objc.GetClass("NSEvent")).Send(selAdd, uintptr(mask), uintptr(blk))
		if mon != 0 {
			outsideMonitor = mon.Send(selRetain)
		}
	})
}

func goCFString(ref uintptr) string {
	if ref == 0 || cfGetTypeID(ref) != cfStringGetTypeID() {
		return ""
	}
	buf := make([]byte, 256)
	if !cfStringGetCString(ref, &buf[0], len(buf), utf8Encoding) {
		return ""
	}
	return cString(&buf[0])
}

func cString(p *byte) string {
	if p == nil {
		return ""
	}
	var b []byte
	const maxCStringLen = 4096
	for ptr := unsafe.Pointer(p); *(*byte)(ptr) != 0; ptr = unsafe.Add(ptr, 1) {
		b = append(b, *(*byte)(ptr))
		if len(b) >= maxCStringLen {
			break
		}
	}
	return string(b)
}
