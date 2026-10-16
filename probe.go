package main

import (
	"fmt"
	"time"

	"github.com/binaryify/MonitorBar/internal/display"
	"github.com/binaryify/MonitorBar/internal/mitv"
)

// probe prints what the app can see and control, without UI (--probe).
func probe() {
	useSystemLanguage(systemLocale())
	m := display.Target()
	fmt.Printf(tr("显示器: %s  接口: %s\n", "Display: %s  Connection: %s\n"), m.Name, or(m.Connection, tr("未知", "unknown")))
	current, ok := m.CurrentMode()
	if ok {
		defer current.Release()
		hidpi := ""
		if current.HiDPI() {
			hidpi = " HiDPI"
		}
		fmt.Printf(tr("当前模式: %s%s %dHz\n", "Current mode: %s%s %dHz\n"), current.Title(), hidpi, current.RefreshRate)
	}
	modes := m.Modes()
	defer func() {
		for _, mode := range modes {
			mode.Release()
		}
	}()
	w, h := m.NativeSize(modes, current)
	fmt.Printf(tr("原生分辨率: %d × %d（%s）\n", "Native resolution: %d × %d (%s)\n"), w, h, display.SizeLabel(w, h))
	fmt.Printf(tr("可用模式: %d 个  亮屏: %v  空闲: %.0f 秒\n", "Modes: %d  Awake: %v  Idle: %.0fs\n"), len(modes), !m.Asleep(), display.IdleSeconds())

	if d, err := m.OpenDDC(); err != nil {
		fmt.Println(tr("DDC/CI: 显示器未响应", "DDC/CI: no answer"))
	} else {
		for _, v := range []struct {
			code byte
			name string
		}{{display.VCPBrightness, tr("亮度", "Brightness")}, {display.VCPContrast, tr("对比度", "Contrast")}, {display.VCPVolume, tr("音量", "Volume")}, {display.VCPInput, tr("输入源", "Input")}} {
			if cur, max, ok := d.Get(v.code); ok {
				fmt.Printf("DDC/CI %s: %d / %d\n", v.name, cur, max)
			}
		}
		fmt.Println(tr("DDC/CI 能力:", "DDC/CI capabilities:"), or(d.Capabilities(), tr("（未提供）", "(none)")))
		d.Close()
	}

	start := time.Now()
	prefs := loadSettings()
	host := prefs.NetworkHost
	link := mitv.New(host)
	name, err := link.DeviceName()
	if host == "" || err != nil {
		host, err = mitv.Discover()
		if err != nil {
			fmt.Println(tr("局域网: 未发现澎湃 OS 显示器", "LAN: no HyperOS display found"))
			return
		}
		link = mitv.New(host)
		name, _ = link.DeviceName()
		prefs.NetworkHost = host
		prefs.save()
	}
	fmt.Printf(tr("局域网: %s @ %s（%.2fs）\n", "LAN: %s @ %s (%.2fs)\n"), name, host, time.Since(start).Seconds())
	if sources, err := link.Sources(); err == nil {
		fmt.Print(tr("信号源:", "Inputs:"))
		for _, s := range sources {
			fmt.Printf(" %s=%d", s.Name, s.ID)
		}
		fmt.Println()
	}
	if v, err := link.Volume(); err == nil {
		fmt.Printf(tr("音量: %d\n", "Volume: %d\n"), v)
	}
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
