<p align="center"><img src="resources/icon.png" width="128" alt=""></p>

<h1 align="center">MonitorBar</h1>

<p align="center">Mac 菜单栏显示器管理：输入源、音量、亮度、色温、刷新率，一页面板<br>
A simpler monitor control for the Mac menu bar — <a href="#english">English</a></p>

<p align="center">
<img src="docs/screenshots/panel.png" width="270" alt="面板">
</p>

## 是什么

为 REDMI 显示器（没有 DDC/CI）做的菜单栏工具：通过澎湃 OS 的局域网遥控接口切换输入源（HDMI / DP / USB-C）和调音量；支持 DDC/CI 的显示器走硬件背光与画质项；任何显示器都能用显卡 Gamma 表做软件调光与色温。一页面板按使用频率排列：场景（编程 / 阅读 / 影音）、画面、音量、输入源、刷新率，显示器回应的 DDC/CI 画质项按需显示。

## 和原版的区别

基于 [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar) 大幅简化重写。原版是多标签的全家桶，这里做减法，只留一页面板：

| | 原版 | 本仓库 |
| --- | --- | --- |
| 界面 | 四个标签页 | 一页面板 |
| 屏幕使用时间 | 有 | 无 |
| 自动场景 / 日夜切换 | 有 | 无 |
| 自动更新 | 有 | 无 |

单个 .app（约 12 MB），无守护进程、无遥测、不收集数据。目前只发布 Apple 芯片版。

## 下载

[Releases](../../releases) 提供 Apple 芯片 dmg。未做 Apple 开发者签名，首次打开右键 → 打开；第一次连接显示器时允许「查找本地网络上的设备」。开机自启：面板「⋯」→「登录时启动」。

## 命令行

```bash
MonitorBar --show                 # 打开面板
MonitorBar --probe                # 打印显示器、接口、DDC/CI 读数和局域网发现结果
MonitorBar --open-at-login on     # 开机自启（off 关闭）
```

## 构建

Go 1.27+，界面基于 [mygo](https://mygo.egoist.dev/)。

```bash
go tool mygo dev                                # 开发，改动后自动重启
go tool mygo build -platform darwin/arm64       # 打包
go test ./...                                   # 测试
```

适配其他显示器：先跑 `--probe` 看识别结果；DDC/CI 编码在 `internal/display/ddc.go`（输入源名称 `InputName`，面板项在 `store.go` 的 `controlCatalog`）；其他局域网接口可参照 `internal/mitv` 写新通道。

## English

MonitorBar puts monitor control in the Mac menu bar — input switching (HDMI / DP / USB-C), volume, brightness and warmth, refresh rate — on one panel. It drives HyperOS monitors over their LAN remote API (they lack DDC/CI), uses DDC/CI where available, and does software dimming through the GPU gamma table on any display.

This is a much simpler rewrite of [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar): no tabs, no screen-time tracking, no auto-update, no scheduling — just the panel. Apple Silicon dmg on the [Releases](../../releases) page. Not signed with an Apple Developer ID: right-click → Open on first launch.

## License

[MIT](LICENSE) · 原项目 [Binaryify/MonitorBar](https://github.com/Binaryify/MonitorBar)（MIT），感谢原作者。
