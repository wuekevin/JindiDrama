---
title: 'E12 媒体读空闲后的原生恢复验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '4a454079198ac212c3b473fd9f21d9d0bc0ca3cf'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
followup_review_recommended: false
context: []
warnings: []
deferred: []
---

<intent-contract>

## Intent

确认媒体请求在生产的 12 秒空闲上限后中断时，播放页能在原生 WebView 切换可用备用源。若全失败，应显示可重试错误，重新取票后可以播放，同时保留已有观看进度。对收到截断网络流的实际播放器补回归，并记录平台与数据验证范围。

</intent-contract>

## Implementation Notes

先用 Wails 测试应用替换上游传输，保留生产票据、Go 转发和 WebView 行为。当前 Wails 版本通过两种原生验收。本轮增补真实断流的 Chromium 回归。

## Plan Change Log

## Review Triage Log

### 2026-11-27 — Review pass

- verdicts: 2 findings — high 0, medium 1, low 1, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` 原始界面自动化在播放后读取 6 秒，无法排除从 0 开始。在同一源码的 Wails 夹具增加只读 seeked 探针，隔离 profile 与日志，验证点击播放前位置为 5 且仍暂停，再确认退出后 SQLite 推进到 6465 毫秒。
  - `[low]` `[patch]` 原生试验替换了外部 HTTP，没有走真实上游 TLS 协商。STATUS 标明原生只覆盖 Go 逐响应超时与 WKWebView，生产 TLS 路径由独立 Go 回归覆盖。

## Verification

- Apple Silicon 上让首选请求在正文中途停住，验证备用读取和实际解码；模拟两路失败并从错误状态重试。
- 使用真实浏览器网络错误验证恢复链路；运行前端类型、生产构建、目录与播放回归、Go 全量及 vet。
- 两轮原生验收记录均通过。重试后隔离 SQLite 的进度从 5000 毫秒推进到 6465，完整性通过。

## Auto Run Result

- Apple Silicon Wails 在生产的空闲上限下，实际从停滞的首选源切到备用源。两路停滞会显示可重试错误与保持原进度。新票据启动后，播放前读取 5 秒，正常退出后播放位置推进，SQLite 完整性通过。
- `15-provider-playback.spec.ts` 补入真实 HTTP 正文截断、两路故障、用户重试、备用源和进度的回归。`docs/STATUS.md` 记录证据及平台限制。
- 快审处理 1 个 medium、1 个 low，补原生播放前断言并澄清测试层级。无延期项。
- 生产构建、类型检查、122 项 Vitest、60 项目录与播放 Playwright、Go vet 及 Go 全量通过。真实授权片源、Windows/Intel Mac 原生和正式签名公证仍待验收。
