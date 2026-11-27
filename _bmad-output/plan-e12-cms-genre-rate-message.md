---
title: 'E12 分类浏览保留苹果 CMS 限流提示'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: 'fd74137c5a65a26f6b89abcca8312b1deee471f4'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

苹果 CMS 分类列表或分类分页收到上游限流时，页面给出已有的稍后重试提示。重试继续当前分类与页码，分页失败时保留上一页已经加载的内容。普通分类错误保留通用文案，播放页延续原有处理。

</frozen-after-approval>

## Implementation Notes

分类适配器会将所有错误改写成通用分类不可用。搜索页的分类分支也会再次改写错误。共用固定限流提示判断，覆盖 Go 桌面桥的字符串及 Error 形态。

## Verification

- 用 Playwright 复现分类列表、首次分类结果及已有结果时的限流提示，核对当前分类与页码、重试恢复和错误文案。
- 前端类型检查、Vitest、Chromium 目录与播放限流、WebKit 专项。查看页面限流和保留旧结果的显示。

## Results

- Chromium 原版复现把 Go 风格字符串限流误报为分类暂不可用。修复后保留分类列表和分类结果的限流说明，无已加载结果及保留上一页时均显示原因。重试继续原分类和请求页码。
- 前端类型检查与生产构建、Vitest 125 项、Chromium 目录 20 项和播放限流 1 项通过。WebKit 分类与播放 3 项通过。对照 1448×1086 原版、限流提示和保留上一页画面。最小窗口 1280×860 及 220% 缩放下，提示正常折行，重试按钮可见。
- Apple Silicon 测试构建使用真实 Go、SQLite、Wails 和 WKWebView，替换隔离 CMS 传输并注入按钮观察脚本。分类和第 2 页各收到一次 429。冷却期重试无额外上游请求，冷却后恢复到第 2 页。正常退出，SQLite 完整性通过。授权公网片源待联调。

## Review Triage Log

- 低，已修复。「全部分类」保留上一页时也使用同一分类提示规则。普通错误保留通用文案，限流保留专用提示。新增普通错误分页回归。
- 低，已修复。分类列表尚未加载时显示「所选分类暂未加载」，保留原值。列表加载后显示正常标签。Chromium、WebKit 与原生探针均核对初始选择。
