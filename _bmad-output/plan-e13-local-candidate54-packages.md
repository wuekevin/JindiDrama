---
title: 'E13 .54 累计本地候选'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'b41f03e8136d6b6f850bec806a2e5ca66efb0762'
route: oneshot
route_source: auto
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

将 .53 后的 Windows 备份权限、SQLite 文件句柄快照和输入预检合并成 .54。使用同一生产前端生成 Windows x64 安装器及便携版和 macOS Universal ZIP。从本地成品复核结构、签名类型、SHA-256 及 Apple Silicon 可运行的回滚与 Wails 文档就绪。

</frozen-after-approval>

## Implementation Notes

重新核对 .53 到当前发行源码的前端差异及实际生产资源摘要。原生 GUI 能力以系统锁屏状态为准。

## Plan Change Log

## Review Triage Log

## Verification

- 核对 Windows 主程序与安装器载荷、WebView2 引导程序和 PE 版本。
- 核对 macOS 双切片部署版本、临时签名、归档内 SQLite 回滚及就绪标记。
- 重新生成和独立检查 13 项 SHA-256。核对实际成品的生产依赖扫描。

- 当前 Go 全量、vet、备份专项与竞态已通过。前端与 .53 源码及生产资源一致，复用对应前端验收。
- 校验 13 项 SHA-256，Windows NSIS 内主程序与 WebView2 引导程序匹配。Universal 两个架构最低为 macOS 13.0。三个主程序切片、Go 程序源码及 npm 生产依赖扫描均未报告已知漏洞。
- 锁屏下验证当前归档内程序的实际 WKWebView 文档就绪和正常退出，可见窗口和系统选择器待另验。从归档内程序复核真实备份生成 v2、拒绝伪装备份和保护源文件。
- Windows 和 Intel Mac 原生测试、正式签名、公证、在线更新及授权接口联调待完成。详细日志见 [STATUS.md](../docs/STATUS.md)。
