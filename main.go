// MonitorBar manages an external monitor from the macOS menu bar: inputs and
// volume over the HyperOS LAN API, hardware/software brightness and warmth,
// refresh rates, and DDC/CI controls.
//
//	MonitorBar           run, panel opens on first launch
//	MonitorBar --show    run and open the panel
//	MonitorBar --probe   print what it can see and control, then exit
//	MonitorBar --open-at-login on|off   start at login or not, then exit
package main

import (
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/egoist/mygo"
)

func main() {
	if slices.Contains(os.Args[1:], "--probe") {
		probe()
		return
	}
	// Render the compact menu bar flyout via CPU-backed CALayer on macOS so
	// opening/closing the popover never creates or tears down a CAMetalLayer
	// swapchain (which can cause WindowServer/DCP to flicker high-refresh
	// HDMI/VRR displays or momentarily bypass custom gamma LUTs).
	if os.Getenv("MYGO_GPU") == "" {
		_ = os.Setenv("MYGO_GPU", "0")
	}
	mygo.App.SetName("MonitorBar")
	if i := slices.Index(os.Args, "--open-at-login"); i > 0 && i+1 < len(os.Args) {
		setOpenAtLogin(os.Args[i+1] == "on")
		return
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {}) // the panel is the only window: keep running as it closes
	a := &app{}
	show := slices.Contains(os.Args[1:], "--show")
	mygo.App.WhenReady(func() { a.ready(show) })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// setOpenAtLogin is a command: the login item needs the app's event loop,
// so it runs one, changes the setting, reports it and quits.
func setOpenAtLogin(on bool) {
	useSystemLanguage(systemLocale())
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.WhenReady(func() {
		if err := mygo.App.SetOpenAtLogin(on); err != nil {
			fmt.Println(tr("开机自启设置失败:", "Open at login failed:"), err)
		}
		fmt.Println(tr("开机自启:", "Open at login:"), map[bool]string{true: tr("已开启", "on"), false: tr("已关闭", "off")}[mygo.App.OpenAtLogin()])
		mygo.App.Quit()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
