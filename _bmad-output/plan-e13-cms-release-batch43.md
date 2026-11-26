---
title: 'E13 本机互动与回滚合并验收批次'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '565e75077adfb62a192ed578761b42f8b1537e69'
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

用同一发行源码合并上次测试包后的 CMS 本机互动、快捷键、高缩放、发行草稿核验和回滚改进。生成 Windows 安装与便携包和 macOS Universal 测试包，记录回归、成品校验、原生和授权范围。

</frozen-after-approval>

## Implementation Notes

在独立检出使用同一生产文件做测试和打包。校验随包文档、SHA-256、Windows 安装器资源、macOS 两个架构和 Apple Silicon 从最终归档运行。

## Plan Change Log

## Review Triage Log

## Verification

- 前端生产构建、全量 Playwright 和 Vitest、Go vet 与全量、发行辅助工具、源码与二进制依赖扫描。
- 逐项复验两平台校验清单，检查归档平台、Windows 安装器内容、Apple Silicon 成品启动、退出和回滚。

## Auto Run Result

- Windows 与 macOS 共 13 项 SHA-256 通过。Windows 安装器内容、macOS 双架构及 13.0 部署目标符合预期。
- 69 个前端生产文件哈希一致。378 项 Playwright、122 项 Vitest、Go 测试、13 项 Node 与 13 项 Python 工具测试通过。依赖扫描无报告。
- Apple Silicon 从最终 ZIP 实测启动、退出和回滚。对填充过的测试库复验 v10 → v2，旧版数据层可读取原有记录。
- Hosted Windows、Intel Mac、正式签名与公证、真实授权接口仍待验收。本轮只执行本地打包。
