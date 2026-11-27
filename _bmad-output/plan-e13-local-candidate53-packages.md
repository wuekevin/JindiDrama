---
title: 'E13 .53 累计本地候选'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e506ab1d7667ad5dc875e2a3edb68501b90baa56'
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

从同一发行源码累计验证 SQLite 备份创建与恢复、历史精度、ExFAT 媒体保存和回滚权限及发布。生成 Windows x64 和 macOS Universal 本地测试包，自行核对安装和归档文件的 SHA-256。用 Apple Silicon 运行归档内程序，核对原生启动、退出和回滚。

</frozen-after-approval>

## Implementation Notes

使用 Node 22 按锁文件重新安装依赖，并复用本机打包工具。前端重新构建，再核对浏览器与两个桌面打包环节的同一产物。以隔离标识和临时签名启动 macOS ZIP。

## Plan Change Log

## Review Triage Log

## Verification

- 前端类型检查、Vitest、完整 Chromium 和 WebKit 专项、Go 测试及依赖检查。
- Windows NSIS 内容、便携程序和随包校验命令。Universal 两切片部署目标、签名、ZIP 和随包校验命令。
- Apple Silicon 从归档启动，核对 SQLite 完整性和 v2 回滚。两个平台归档后再复核清单。

- 实跑通过 125 项 Vitest、394 项 Chromium、4 项 WebKit 专项及 Go 全量、vet 和竞态检查。源码和最终二进制扫描未报告已知漏洞。
- 随包校验命令与归档后的独立 SHA-256 检查通过，Windows 7 项、macOS 6 项。包结构及 Apple Silicon 原生检查通过。运行 ZIP 内的同一程序生成有效 SQLite v2，排除了验收夹具预先设置的继承 ACL。
- 本轮覆盖本地 smoke 成品。Windows 和 Intel Mac 当前版本原生验收、正式分发凭据及授权公网接口联调待完成。详细日志见 [STATUS.md](../docs/STATUS.md)。
