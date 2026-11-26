---
title: 'E12 收藏和历史中的苹果 CMS 核验重试'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '810c04acecddca779a64055a92b8bdc5bfdff669'
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

苹果 CMS 临时核验失败后，收藏和历史原地恢复。只重新核验用户选择的失败条目，沿用共享并发上限和离页处理。一时的上游故障不影响已存收藏与断点。

</frozen-after-approval>

## Implementation Notes

复用失败条目当前的操作位置和现有视觉控件。仅在上游详情调用报错时提供重试。

## Plan Change Log

## Review Triage Log

## Verification

- 两个页面用真实浏览器验证上游临时不可用、单条重试和断点恢复。检查分页、共享并发及首页原有重试。
- 对照收藏、历史的设计稿核对控件位置和缩放表现。

## Auto Run Result

- 两页均可原地重试失败条目，复用原目录并发槽和现有卡片按钮，保留收藏和进度。
- 两项新增回归在旧代码失败。生产构建、122 项 Vitest、63 项目录及播放和 25 项收藏与历史相关回归通过。
- 截图复核收藏与历史设计稿的主要操作位置。未触发 GitHub 打包或使用真实授权片源。
