---
title: 'E13 正式 macOS 归档分发策略检查'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '3ed81709ab00189d87d4bcf17fd9223e87f12176'
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

正式 macOS ZIP 解包后，在原生启动前检查分发策略，并保留工具诊断。Apple Silicon 与 Intel 使用同一入口。测试包继续验证现有代码签名和启动流程。

</frozen-after-approval>

## Implementation Notes

在已有归档冒烟中复用签名与公证校验。现代 macOS 使用 syspolicy_check，旧系统使用 spctl。

## Plan Change Log

## Review Triage Log

## Verification

- 回归分别覆盖测试包、正式检查、旧系统和拒绝传播。
- 从现有 .41 ZIP 解包，用系统工具确认临时签名会被分发策略拒绝，再运行普通原生冒烟验证退出与 SQLite。

## Auto Run Result

- 正式归档解包后、公证票据通过后执行系统策略检查。Apple Silicon 与 Intel 共用入口，保留失败输出。
- 13 项 Python 测试通过。macOS 实际工具拒绝原 .41 归档的临时签名。独立 bundle ID 副本在普通模式从归档启动，正常退出且 SQLite 完整；正式模式因无票据失败并保留诊断。
- 正式签名及公证成功路径待持证书实跑。本轮未触发 GitHub 打包。
