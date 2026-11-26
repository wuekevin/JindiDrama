---
title: 'E12 苹果 CMS 分集本机评论'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '830bb45ff66f9c953554d8043bfd1a415e7ee970'
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

为苹果 CMS 播放页提供按片源、短剧和分集隔离的本机评论。使用既有 SQLite、窗口关闭写入和恢复机制。从空列表开始，显示真实本机计数，评论文本不发送给片源。

</frozen-after-approval>

## Implementation Notes

复用评论面板和持久化接口。Go 对本机评论核对启用片源标识和安全分集范围，前端从实际上游可播分集呈现入口。

## Plan Change Log

## Review Triage Log

## Verification

- 真实 SQLite 测试核对分集隔离、重启、片源身份和零上游请求。
- 浏览器验证空状态、保存、换集与刷新，回归演示评论，检查新增面板的截图。

## Auto Run Result

- 新增按片源、短剧和分集隔离的本机评论入口，复用面板、SQLite 和既有保存恢复机制。
- 102 项 Playwright、122 项 Vitest、Go vet、全量、评论竞态及 Windows amd64 交叉构建通过。Apple Silicon 原生 Wails 在片源故障模式下仍能保存，无新增目录请求，重启后评论保留且与第 2 集隔离。
- 对照评论设计稿复用编辑和排序控件。CMS 分集面板放在选集下方，以继续使用原生视频的既有宽度。
