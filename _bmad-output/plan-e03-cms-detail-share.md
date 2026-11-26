---
title: 'E03 苹果 CMS 短剧详情分享'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '209045b04dde381c79b1fc6e00675f11ce6cba79'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

在苹果 CMS 剧目详情复制桌面分享链接。链接仅含剧目标识，接收方用已配置的同一片源打开详情。链接不携带片源地址、票据、账号或播放进度。未知、过期或异源剧目沿用现有详情错误，链接不自动播放。

</frozen-after-approval>

## Implementation Notes

扩展已有分享解析和剪贴板入口，复用 Provider 的同源详情约束。仅允许 CMS 详情深链。

## Plan Change Log

## Review Triage Log

- 中 · 对同一剧目重开链接时原路由可复用，无法感知剧目已下架。添加同页刷新，回归覆盖当前页再次打开后显示未找到状态。
- 中 · 原生 Wails 的未找到错误是 Error 对象，浏览器夹具早先传字符串。统一识别两种形态，再复验原生异源链接。

## Verification

- Go 深链正反例、前端构建和详情、分享与 CMS Playwright。最小窗口 220% 下截图核对分享按钮。

## Auto Run Result

- 生产前端、122 项 Vitest、129 项详情和授权片源回归（切源时序用例单独复跑通过）、Go vet 与 Go 全量及分享竞态通过。对照详情设计稿复核 220% 截图。
- Apple Silicon 使用本机合成片源验证系统深链，同源打开详情、异源无上游详情及媒体请求。Windows x64 编译通过，Windows 原生协议待下一批验收。
