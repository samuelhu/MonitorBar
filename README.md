<p align="center"><img src="resources/icon.png" width="128" alt=""></p>

<h1 align="center">MonitorBar</h1>

<p align="center">Mac 菜单栏显示器管理：输入源、音量、亮度、色温、刷新率，一页面板<br>
A simpler monitor control for the Mac menu bar — <a href="#english">English</a></p>

<p align="center">
<img src="docs/screenshots/panel.png" width="270" alt="面板">
</p>

## 是什么

为 REDMI 显示器（没有 DDC/CI）做的菜单栏工具：通过澎湃 OS 的局域网遥控接口切换输入源（HDMI / DP / USB-C）和调音量；支持 DDC/CI 的显示器走硬件背光与画质项；任何显示器都能用显卡 Gamma 表做软件调光与色温。

一页面板，按使用频率排列：场景（编程 / 阅读 / 影音）、画面、音量、输入源、刷新率；显示器回应的 DDC/CI 画质项按需显示。

## 和原版的区别

基于 [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar) 大幅简化重写。原版是多标签的全家桶，这里做减法，只留一页面板：

| | 原版 | 本仓库 |
| --- | --- | --- |
| 界面 | 四个标签页 | 一页面板 |
| 屏幕使用时间 | 有 | 无 |
| 自动场景 / 日夜切换 | 有 | 无 |
| 自动更新 | 有 | 无 |

单个 .app（约 12 MB），无守护进程、无遥测、不收集数据。目前只发布 Apple 芯片版。

## 场景

三个场景是舒适起点，不是校准档。**亮度百分比以显示器自身亮度 100% 为基准设计**：用软件调光的显示器（如 REDMI）请把 OSD 亮度保持在 100%，亮度交给面板调节，百分比才与场景设计一致；支持 DDC/CI 的显示器由应用直接控制背光，无需手动设置。

| 场景 | 亮度 | 冷暖 | 白点（设计值） | 适合 |
| --- | --- | --- | --- | --- |
| 编程 Coding | 65% | 10% | ≈ 5800K | 长时间写代码：稍暖、略降亮度 |
| 阅读 Reading | 50% | 25% | ≈ 4900K | 文字阅读：更暖、更暗 |
| 影音 Movie | 保持当前 | 归零 | D65（6500K） | 影音娱乐：中性白，不动亮度 |

实现上，冷暖通过显卡 Gamma 表的 RGB 增益完成：红通道不变，绿随冷暖小幅下降，蓝降幅最大；上表白点为设计目标（按 mired 换算），非实测校准值。亮度百分比同样不对应固定 cd/m²——它们描述的是相对明暗，不是绝对亮度。

## 安装

1. 到 [Releases](../../releases) 下载最新的 `MonitorBar-*-macos-arm64.dmg`（需要 Apple 芯片 Mac）。
2. 打开 dmg，把 **MonitorBar** 拖进「应用程序」。
3. 首次打开会被 macOS 拦截——应用未签名、未公证，属预期。**先双击一次** MonitorBar，然后打开 **系统设置 → 隐私与安全性**，滚动到底部，点「**仍要打开**」（Open Anyway），输入密码确认。「仍要打开」按钮在拦截之后才会出现。（macOS 14 及更早版本：右键 → 打开 即可。）
4. 第一次连接显示器时，允许「**查找本地网络上的设备**」——切换输入源、调音量都靠它。

可选：面板右上角「⋯」→「登录时启动」，开机自动运行。

## 构建

Go 1.27+，界面基于 [mygo](https://mygo.egoist.dev/)。

```bash
go tool mygo dev                                # 开发，改动后自动重启
go tool mygo build -platform darwin/arm64       # 打包
go test ./...                                   # 测试
```

适配其他显示器：先运行 `go run . --probe` 看识别结果（已安装的 app 也可运行 `.app/Contents/MacOS/MonitorBar --probe`）；DDC/CI 编码在 `internal/display/ddc.go`（输入源名称 `InputName`，面板项在 `store.go` 的 `controlCatalog`）；其他局域网接口可参照 `internal/mitv` 写新通道。

## English

MonitorBar puts monitor control in the Mac menu bar — input switching (HDMI / DP / USB-C), volume, brightness and warmth, refresh rate — on one panel. It drives HyperOS monitors over their LAN remote API (they lack DDC/CI), uses DDC/CI where available, and does software dimming through the GPU gamma table on any display.

A much simpler rewrite of [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar): no tabs, no screen-time tracking, no auto-update, no scheduling.

**Scenes** are comfort starting points designed against the display's own brightness at **100%** — with software dimming (e.g. REDMI monitors), leave the OSD brightness at 100% and dim from the panel; with DDC/CI the app drives the backlight directly. Coding: 65% · warmth ≈ 5800K. Reading: 50% · ≈ 4900K. Movie: keeps your brightness, neutral D65 white. Warmth is applied as RGB gains in the gamma table — the Kelvin figures are design targets, not measured colorimetry.

**Install:** download the dmg from [Releases](../../releases) (Apple Silicon), drag MonitorBar to Applications. The app is unsigned and not notarized, so macOS blocks the first launch — that's expected. Double-click once, then open **System Settings → Privacy & Security** and click **Open Anyway** at the bottom (on macOS 14 and earlier, right-click → Open instead). Allow local-network access when prompted, and optionally enable launch-at-login from the panel's ⋯ menu.

## License

[MIT](LICENSE) · 原项目 [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar)（MIT），感谢原作者。
