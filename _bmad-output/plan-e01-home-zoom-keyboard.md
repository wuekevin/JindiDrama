---
title: 'E01 首页大缩放的键盘浏览'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '33274ee0a8c3685b2684bcbed29385a7a1717906'
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

在项目支持的最小桌面窗口 1280×860、220% 缩放时，首页题材、上新、排行榜与继续观看要在内容区可读和可操作。用 Tab 浏览至继续观看时，已聚焦卡片应完整或近乎完整可见，主内容不会被横向推移。1448×1086 的默认首页保持原设计。

</frozen-after-approval>

## Implementation Notes

使用首页内容容器宽度调整列数。出现问题时实际内容滚动容器的 scrollLeft 非零，继续观看卡片只显示约 5%。

- 用首页局部容器查询调整三组卡片。旧版卡片可见比率 0.052，修复后 Chromium 和 macOS WebKit 都超过 0.90，内容区横向滚动值为零，Enter 打开分集。
- Chromium 首页回归 21 项、十处主要 UI 的键盘检查、122 项 Vitest 和 WebKit 专项通过。1448×1086 前后截图字节一致。

## Plan Change Log

## Review Triage Log

## Verification

- 前端类型检查、生产构建和 Vitest 通过。
- 首页键盘专项真实运行 Tab 到继续观看，确认可见并按 Enter 打开播放。
- 1280×860、220% 缩放验证十处主要 UI 入口全部获得可见焦点。
- 比对 1448×1086 默认首页截图和 1280×860 聚焦前后截图。
