<p align="center"><img src="resources/icon.png" width="128" alt=""></p>

<h1 align="center">MonitorBar</h1>

<p align="center">macOS 菜单栏 / Windows 托盘里的显示器管理工具，一份 Go 代码，两端原生界面。<br>
A menu bar (macOS) and tray (Windows) app to manage your monitor — <a href="#english">English</a></p>

<p align="center">
<img src="docs/screenshots/panel.png" width="270" alt="面板">
<img src="docs/screenshots/windows.png" width="270" alt="Windows 11">
</p>

## 为什么做这个

REDMI 显示器不支持 DDC/CI。

Mac 上常见的显示器工具（MonitorControl、BetterDisplay、Lunar 等）都靠 DDC/CI 控制显示器，所以在 REDMI 显示器上切换不了输入源，也调不了音量。

不过 REDMI 显示器内置澎湃 OS，开机时会在局域网提供一个遥控接口。MonitorBar 通过这个接口切换输入源（包括 DP 和 USB-C）、调节音量，不依赖 DDC/CI。软件调光、色温和刷新率切换也一并做了进去，并且有 Mac 和 Windows 两个版本：两台电脑接在同一台显示器上时，在任意一台上都能一键切到另一台。

MonitorBar 只在 REDMI 显示器 G Pro 27U 2026 上测试过，不保证在你的显示器上能正常工作：不同型号的接口、信号源编号可能不一样。如果不能用，可以借助 AI 编程工具来适配：先运行 `MonitorBar --probe` 看看它识别到了什么，再让 AI 参照下面的[适配其他显示器](#适配其他显示器)一节修改代码。

## 功能

一页面板，按使用频率从上到下：

| 区域 | 功能 |
| --- | --- |
| 场景 | **编程 / 阅读 / 影音** 三个可调整的舒适起点 |
| 画面 | 软件亮度（或硬件背光）、冷暖色温 |
| 音量 | **显示器音量**（局域网或 DDC/CI） |
| 输入源 | **切换 HDMI / DP / USB-C 等信号源** |
| 刷新率 | 160 / 144 / 120 / 60 Hz 等常用档位 |
| 显示器 | 对比度、锐度、RGB 增益等 DDC/CI 画质项（显示器回应时才显示） |

- **切换输入源、调音量不需要 DDC/CI**：澎湃 OS 显示器（如 REDMI 显示器 G Pro 27U 2026）在局域网提供遥控接口，MonitorBar 自动发现并使用它。Mac 和 Windows 接在同一台显示器的不同接口上时，两边都能一键切到对方。
- **支持 DDC/CI 的显示器**：自动用 DDC/CI 调硬件背光，切换输入源、调音量，并显示对比度、锐度、RGB 增益等显示器回应的设置。
- **软件调光与色温**通过显卡的 Gamma 表实现，任何连接线、任何显示器都能用。
- 场景是可调整的舒适起点，不是校准档或健康建议。亮度百分比不对应固定的 cd/m²；冷暖控制只调整 Gamma 表中的 RGB 增益，不会测量或保证实际色温。影音场景恢复中性色调并保留当前亮度。中性也不代表显示器已经校准到 [sRGB 的 D65 参考白点](https://www.w3.org/Graphics/Color/srgb)。
- 冷暖控制适合按个人观感调整，不应宣称能预防眼疲劳；[Cochrane 对蓝光过滤眼镜的综述](https://www.cochranelibrary.com/cdsr/doi/10.1002/14651858.CD013244.pub2/full)没有发现其能减轻电脑使用造成的眼疲劳。该综述研究的是眼镜，不是本应用的 Gamma 调整。
- 显示器不支持的功能不会出现在界面上。
- 单个可执行文件，无 webview，空闲时不占 CPU；中文界面，深色主题。

## 安装

从 [Releases](https://github.com/binaryify/MonitorBar/releases) 或 [CI 产物](https://github.com/binaryify/MonitorBar/actions) 下载：

- **macOS**（Apple Silicon / Intel，macOS 12+）：打开 `.dmg`，把 MonitorBar 拖进「应用程序」。应用未签名，首次打开需右键 → 打开。第一次连接显示器时允许「查找本地网络上的设备」。
- **Windows 10 / 11**：把 `MonitorBar.exe` 和同目录的 `icon.png` 放到任意文件夹（推荐 `%LOCALAPPDATA%\Programs\MonitorBar`）后运行。托盘图标默认在 `^` 溢出区，可拖到任务栏常驻。

开机自启：默认关闭，由用户自己选择开启——面板右上角「⋯」→「登录时启动」，或命令行 `MonitorBar --open-at-login on`（`off` 关闭）。

## 命令行

```bash
MonitorBar --show                 # 启动并打开面板（程序已运行时，再次启动也会打开面板）
MonitorBar --probe                # 打印显示器、接口、分辨率、DDC/CI 读数和局域网发现结果
MonitorBar --open-at-login on     # 开机自启（off 关闭）
```

## 工作原理

MonitorBar 有三条控制显示器的通道，启动时在后台探测，哪条可用就用哪条：

| 通道 | 能做什么 | 条件 |
| --- | --- | --- |
| 局域网遥控 | 切换输入源、调音量 | 澎湃 OS 显示器，与电脑在同一网络 |
| DDC/CI | 硬件背光、输入源、音量、对比度等 | 显示器支持 DDC/CI（部分需在显示器菜单开启） |
| 显卡 Gamma 表 | 软件调光、色温 | 任何显示器 |

输入源和音量优先走局域网，其次 DDC/CI；亮度有 DDC/CI 时调硬件背光，否则用软件调光。

- **局域网遥控**：澎湃 OS 的 6095 端口提供 `getsources`（信号源列表）、`changesource2`（按编号切换，能切到 DP / USB-C）、`getvolume`、`keyevent`（遥控按键，音量 ±1）。首次启动扫描所在 /24 网段找到显示器，之后记住地址。
- **DDC/CI**：macOS 通过 IOAVService 读写显示器的 I2C 总线（Apple Silicon）；Windows 通过系统的 Monitor Configuration API（dxva2）。输入源列表取自显示器的 MCCS 能力字符串。
- **macOS**：通过 [purego](https://github.com/ebitengine/purego) 调用 CoreGraphics、IOKit、AppKit（无 cgo）。
- **Windows**：通过 `syscall` 调用 user32、gdi32、dxva2、powrprof。

## 平台差异

- **软件调光**：Windows 只接受离标准曲线不远的 Gamma（最暗约 50%），更暗时退到允许的最暗并保留色温比例，面板会提示。macOS 无此限制。退出时两端都会恢复原始曲线。
- **面板位置**：从溢出区打开时面板贴在图标上方；拿不到图标位置时（如再次启动程序打开）贴在屏幕右下角。
- 数据保存在 `~/Library/Application Support/MonitorBar`（macOS）或 `%APPDATA%\MonitorBar`（Windows）。

<p align="center"><img src="docs/screenshots/windows.png" width="300" alt="Windows 11 上的面板"><br><sub>Windows 11 上的面板</sub></p>

## 开发

基于 [mygo](https://mygo.egoist.dev/)，需要 Go 1.27+（`go` 会自动下载对应工具链）。

```bash
go tool mygo dev                                         # 开发，改动后自动重启
go test ./...                                            # 无窗口的界面测试与单元测试
MONITORBAR_SNAPSHOTS=docs/screenshots go test -run TestSnapshots .   # 重新生成截图
go tool mygo build -platform darwin/universal,windows/amd64          # 打包两端（可在 Mac 上交叉编译 Windows 版）
```

发布：推送版本标签，CI 会打包 macOS dmg 与 Windows zip 并创建 GitHub Release。

```bash
git tag v1.0.0 && git push origin v1.0.0
```

| 路径 | 职责 |
| --- | --- |
| `main.go` / `app.go` | 入口、托盘图标、弹出面板的尺寸与位置 |
| `store.go` | 状态与动作：只在主线程读写，网络请求在 goroutine 中，结果经 `post` 回到主线程 |
| `panel.go` / `content.go` / `widgets.go` / `icons.go` | 界面 |
| `internal/mitv` | 澎湃 OS 局域网接口与发现 |
| `internal/display` | 平台层：显示器信息、分辨率、Gamma、DDC/CI |

## 适配其他显示器

先运行 `MonitorBar --probe`，它会打印显示器名称、原生分辨率、DDC/CI 的读数与能力字符串，以及局域网发现结果。再按情况修改：

- **支持 DDC/CI，但某些设置不对**：VCP 编码在 `internal/display/ddc.go`，输入源名称在 `InputName`，面板显示哪些设置在 `store.go` 的 `controlCatalog`。
- **DDC/CI 没有响应**：先在显示器菜单里找 DDC/CI 开关。macOS 的读写协议在 `internal/display/ddc_darwin.go`，Windows 在 `ddc_windows.go`。注意部分 Mac 的 HDMI 口不传 DDC/CI，可改用 DP 或 USB-C 连接。
- **其他品牌的局域网接口**：参照 `internal/mitv` 写一个新的通道，在 `store.go` 里接入输入源和音量的路由（`sources`、`selectInput`、`setVolume`）。

## 致谢

- [mygo](https://github.com/egoist/mygo)：Go 桌面应用框架
- [LGwebOS-menubar](https://github.com/gabrielgreco1/LGwebOS-menubar)：界面设计的灵感来源
- 图标风格参考 [Lucide](https://lucide.dev)

## English

REDMI monitors do not support DDC/CI, so the usual tools (MonitorControl, BetterDisplay, Lunar) cannot switch their inputs. They run HyperOS, though, whose LAN remote API can, DP and USB-C included. MonitorBar manages an external monitor from the macOS menu bar or the Windows notification area. It switches inputs and changes volume over the LAN remote API of Xiaomi HyperOS monitors, or over DDC/CI on monitors that support it (backlight and picture controls too), dims and warms the picture through the gamma table, and switches refresh rates. It is one Go codebase built with [mygo](https://mygo.egoist.dev/), drawing native UI on both platforms; the Windows build cross-compiles from a Mac. See [Development](#开发) for build commands.

The Coding, Reading, and Movie scenes are adjustable comfort starting points, not calibration modes or health recommendations. Brightness percentages do not represent a fixed luminance in cd/m². Warmth applies relative RGB gains through the gamma table; the app does not measure or guarantee a specific color temperature. Movie returns the image to neutral and keeps the current brightness. Neutral does not mean the display is calibrated to [sRGB's D65 reference white](https://www.w3.org/Graphics/Color/srgb). Warmth is a personal preference and should not be described as eye-strain prevention. A [Cochrane review of blue-light-filtering spectacles](https://www.cochranelibrary.com/cdsr/doi/10.1002/14651858.CD013244.pub2/full) found no short-term eye-strain benefit from the lenses it studied; that review did not study this app's gamma adjustment.

## 许可证

[MIT](LICENSE)
