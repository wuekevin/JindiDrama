---
title: 'E12 苹果 CMS 原生播放器停滞恢复'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '8f3542c90c7012db6aa7316ff1cc7001dabdb510'
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

首选源在播放中断开时，WKWebView 可能持续重复请求剩余字节，未及时触发媒体错误。在播放时间持续停止推进时沿用现有切源和错误路径。暂停、播完、旧媒体与过期票据不触发该恢复。

</frozen-after-approval>

## Implementation Notes

以播放时间前进为依据。允许 Go 转发层先尝试完成其 12 秒空闲处理，使用独立的播放器停滞上限。

## Plan Change Log

## Review Triage Log

## Verification

- 用 WKWebView 原生进度和本机截断媒体验证播放中首选源退出后同票据切源、继续播放和 SQLite。
- 浏览器回归验证未前进时切源，暂停期间不切源。

## Auto Run Result

- 实际 WKWebView 的首选 MP4 在播放中截断时，旧候选重复请求字节范围，超过 40 秒仍未切源。
- 增加 20 秒不推进的检测后，合成上游故障两轮均接播备用源，第二轮观察到 13 秒。正常退出保存播放位置，SQLite 完整性通过。
- 生产前端构建、122 项 Vitest 和 67 项目录与播放回归通过。本轮未触发 GitHub 打包。
