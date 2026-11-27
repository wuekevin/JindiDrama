---
title: 'E13 .50 累计候选本地打包'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '33a7600d68d1951e4c8ecaea6b8fffeb35d75454'
route: oneshot
route_source: pinned
risk: medium
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

合并 .49 之后的三项修复为一个候选。验证诊断日志遮蔽、苹果 CMS 下架和媒体错误页处理。构建 Windows x64 安装器及便携版和 macOS Universal ZIP。核对最终文件的 SHA-256、安装包内容，并执行 Apple Silicon 原生验收。

</frozen-after-approval>

## Implementation Notes

隔离构建源码、前端回归和发布工具。用同一份经验证的前端构建产物打包两个平台。测试构建采用独立 macOS bundle 标识。

- 392 项 Playwright、122 项 Vitest、Go vet、全量及五个包的竞态检查通过。源代码、三种二进制架构与 npm 生产依赖扫描未报告已知漏洞。
- 两平台 13 项 SHA-256、Windows 安装器内部文件、Universal ZIP 两种切片最低系统版本和 Apple Silicon 成品启动退出回滚通过。首次说明生成的应用路径已修正，沿用同一份 ZIP。

## Plan Change Log

## Review Triage Log

## Verification

- Node 22 锁文件安装、类型检查、生产构建、Vitest 和 Chromium 全量。
- Go vet、全量及 App、CMS、更新、诊断和开机启动竞态。发行脚本和生产依赖检查。
- Windows PE 与 NSIS 内容核对。Universal ZIP 双架构和最低系统版本核对。两套 SHA-256 清单固定后再复验。
- Apple Silicon 从最终 ZIP 验证启动、退出、SQLite 完整性和回滚。
