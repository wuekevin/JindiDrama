---
title: 'E13 .60 累计本地打包验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '011b4cbcf42f45d0d9d8b74c9b8bfc54ea963c76'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

从当前独立发行源码构建一批 Windows x64 和 macOS Universal 测试包，纳入 .59 后的三项 CMS 数据校验修复。复核实际归档的 SHA-256、三架构内嵌内容和 Apple Silicon 原生启动，确认临时签名包无法通过正式签名验收入口。

</frozen-after-approval>

## Implementation Notes

沿用 Wails 与 NSIS 构建和现有归档脚本。从锁文件重建前端并与 .59 的生产前端对比。该前端已通过 Chromium 全量 402 项及 WebKit 切源专项 6 项。隔离生成目录。对 GitHub 的签名与最近工作流状态进行只读核对。

## Verification

- 比对生产前端摘要、Go 全量与 vet、CMS/App 竞态、前端单测和发行校验脚本。
- 读回 Windows 7 项、macOS 6 项 SHA-256，校验 Windows NSIS 与 macOS 两种切片及最低系统版本。三架构逐路径校验内嵌的生产前端。
- Apple Silicon 从最终归档启动、退出和执行 SQLite/WAL 回滚。正式签名验证拒绝同一临时签名归档。

## Results

- Windows x64、macOS Universal 从记录的同一发行提交构建。归档后独立复算 Windows 7 项、macOS 6 项 SHA-256。Windows NSIS 内容比对与 macOS 双切片最低系统版本检查通过。
- 从锁文件重建的 69 个前端文件与 .59 通过浏览器验收的生产前端逐项一致，也与三架构成品内嵌文件逐路径比对通过。前端单测 125 项、Go 全量、vet、App/CMS 竞态、Node 发行脚本 17 项、macOS 工具测试 16 项及 Windows 签名夹具通过。首次 Go 链接遇到磁盘空间不足，清理隔离生成物后全量重跑通过。
- 从归档启动 Apple Silicon 原生窗口，正常退出，隔离 SQLite 完整性与活动 WAL 回滚通过。验收使用临时签名。将已使用的隔离 WebKit 目录移入证据暂存后，同一归档进入正式模式并由 `stapler validate` 以缺少公证票据拒绝。三个主程序切片和 npm 生产依赖扫描未报告已知漏洞。
- GitHub 只读查询返回打包工作流启用，最近完成批次为 .52。可见签名环境、仓库级 secrets 与 Releases 数量均为 0。本机代码签名身份为 0。正式签名、公证、最新候选跨平台原生及公网联调仍待验收。

## Review Triage Log

- 补明签名与包标识。Windows 安装器和便携版均未签名。macOS 归档使用 `local.jindidrama.archiveqa.candidate60.*` 隔离标识和 ad-hoc 签名。正式模式在公证票据检查处拒绝，未进入后续分发策略检查。
- 明确浏览器证据覆盖范围。.60 的前端字节与 .59 相同，完整 Chromium 和 WebKit 专项来自 .59。本轮重新验收成品、前端单测和 Go 变更。
