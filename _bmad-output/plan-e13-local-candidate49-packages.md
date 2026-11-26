---
title: 'E13 .49 累计候选本地打包'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '9208b80b42e77f4d00e87e77090a5ce59d5566a2'
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

基于当前发行源码，将更新下载错误提示、苹果 CMS 试看权限、HLS 明文兼容性三项累计修复作为一个批次验证。构建 Windows x64 安装器和便携版、macOS Universal ZIP，核对 SHA-256 与包内部结构，并从最终 ZIP 执行 Apple Silicon 原生验证。

</frozen-after-approval>

## Implementation Notes

沿用已验证的本地发行路径。用可追溯的独立检出生成前端，给两平台产物核对同一份前端资源。保留用户工作区的索引与本地改动。

- 391 项 Playwright、122 项 Vitest、Go vet、Go 全量和四个包的竞态复验通过。打包过程中并行 Go 测试的链接错误在本轮打包结束后复跑通过。
- Windows 7 项、macOS 6 项 SHA-256 通过项目校验器、系统指令和随包校验指令。Apple Silicon 从最终 ZIP 验证窗口、正常退出、SQLite 完整性及回滚。四项 Go 扫描与 npm 生产依赖审计未报告已知漏洞。

## Plan Change Log

## Review Triage Log

## Verification

- 前端类型检查、生产构建、Vitest 与 Chromium Playwright 全量。
- Go vet、全量 Go、App/CMS/更新竞态、发行辅助脚本测试与依赖漏洞扫描。
- Windows x64 NSIS、PE 和内嵌 WebView2 核对。macOS 两种架构、最低系统版本和 ZIP 完整性核对。
- 两平台逐项校验 SHA-256，并复验随包说明里的校验命令。Apple Silicon 从最终 ZIP 启动、退出并验证隔离 SQLite 和回滚。
