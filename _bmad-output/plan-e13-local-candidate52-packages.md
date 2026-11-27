---
title: 'E13 .52 累计本地候选'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '57bcf11ab9056565f8b24fa5c3ac3a821d5e4ed4'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

按批次构建 .51 之后的累计修复。确认发布源码的解码状态、CMS 公开封面复用、媒体保存目录保护和 Windows 交互验收脚本进入同一候选。Windows 与 macOS 产物可校验，记录本机能验证和仍待实机验证的内容。

</frozen-after-approval>

## Implementation Notes

从同一发行提交用前端锁文件重建，浏览器和桌面构建使用相同的生产前端。使用隔离标识和临时签名启动 Apple Silicon 版本。

## Plan Change Log

## Review Triage Log

## Verification

- 前端生产构建、Vitest、完整 Chromium 回归及前端产物逐项摘要。
- Go 全量、vet、相关竞态与生产依赖漏洞检查。
- NSIS 解包检查、macOS Universal 两架构与最低系统版本、脚本和独立平台 SHA-256。
- Apple Silicon 从归档启动、退出及检查隔离 SQLite 和回滚。

- 本机 394 项 Chromium、122 项 Vitest、3 项 WebKit、Go 全量和相关竞态通过。13 项清单和 Apple Silicon 原生 ZIP 启动与回滚通过。Windows 和 Intel Mac 原生范围列在状态文档。
