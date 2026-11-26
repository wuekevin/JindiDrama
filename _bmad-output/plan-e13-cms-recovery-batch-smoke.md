---
title: 'E13 CMS 目录和媒体恢复批次本地发行验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '427bc10b10bb331fff4ac6edf4b60f7b9154f046'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

从当前隔离发行源码构建一个本地 Windows amd64 与 macOS Universal 批次，累积目录详情合并、媒体停滞恢复及限流改动。用同一份前端生产构建运行全量页面回归和双平台构建。验证产物内容、SHA-256、可用架构启动及依赖安全，记录仍待验收的正式发行条件。

</frozen-after-approval>

## Implementation Notes

单次本地批次复用现有发行工具。隔离全量浏览器回归和打包源目录，避免截图改写打包检出。

## Plan Change Log

## Review Triage Log

## Verification

- 运行全量 Playwright、Vitest 和 Go，复核生产前端文件 SHA-256 在构建前后相同。
- 分别核对安装器成员、双架构归档及平台清单。在 Apple Silicon 从最终归档执行原生冒烟并验证退出。
- 运行 Go 发行二进制漏洞检查和 npm 生产审计，交付至本地项目目录后重新校验 SHA-256。

## Auto Run Result

- Windows 安装器与便携程序、macOS Universal ZIP 已交付至项目本地目录。项目校验器、独立 shasum 和随包说明的命令均匹配 13 项文件。
- 68 个生产前端文件一致。122 项 Vitest、368 项 Chromium、Go vet、Go 全量和发行工具回归通过。Go 源码、三份程序切片和 npm 生产依赖扫描无报告。
- Apple Silicon 从最终 ZIP 正常启动、退出，隔离 SQLite 完整。真实授权公网片源、Windows WebView2 与 Intel 原生启动、正式签名和公证待验收。本地批次未触发 GitHub 打包。
