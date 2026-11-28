---
title: 'E02/E03 WebKit 搜索及片库操作回归'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '7ff527feb63b159f4ed4c5319498df0409776542'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

**问题**　.64 的双引擎验收中，授权片库的鼠标分集重试成功后失去可继续操作的焦点，授权片库搜索流程偶发 ResizeObserver 布局警告，多个用例还依赖特定浏览器的默认操作。

**目标**　保留鼠标分集重试的真实操作及原焦点断言，保证等待期间用户移开的焦点不被抢回。定位搜索布局警告并处理造成它的布局更新，保留页面现有断点。校正双浏览器回归中的行为差异，执行 WebKit 和 Chromium 全量验收。

</frozen-after-approval>

## Implementation Notes

- 使用 .64 的完整失败记录与未改产物作复现基线。侧栏缩放、平台按键、新标签页和剪贴板测试复用现有定位器和断言。
- 搜索观察器延后一帧后仍在全量复现。两个区域仅测宽的试验也出现警告。对照停用两个旧观察器时 35 次通过，因此使用 CSS 容器查询计算顶部栏和搜索布局，保留原宽度档位与页面错误断言。[WebKit macOS Ventura 支持说明](https://webkit.org/blog/13399/webkit-features-in-safari-16-1/)
- 精确沿用分集重试的鼠标操作。点击时聚焦重试按钮，响应时只在按钮仍持有焦点的情况下移交。新增受控延迟场景验证焦点已移走时保持在片库搜索框。
- 双浏览器专项各 6 项通过。WebKit 搜索结果 150% 和 220% 截图与 .64 逐字节一致。Apple Silicon 独立验证归档通过原生启动、正常退出、SQLite 完整性和回滚。

- 最终 Chromium 全量 410 项一次通过。WebKit 409 项一次通过，演示视频暂停时连续快进用例重试通过。授权片库原搜索用例另在最终构建中连续通过 34 次。Vitest 125 项、类型检查、Go 全量与 vet、Windows x64 交叉编译通过。当前双平台打包记录仍为 .64。
- 暂停后连续两次快进的失败有 2/8 独立复现，媒体停在 22 秒。旧 .64 前端对照 8 次均通过，仍待定位媒体时序并核实改动前后的发生条件。本故事的搜索与片库焦点用例均一次通过。

## Verification

- `npm run typecheck`、`npm test`、`npm run build`。
- 在同一构建上单独运行 Chromium 和 WebKit 全量回归，并通过重复运行验证原搜索警告和鼠标重试用例。保持真实视频播放、页面错误和焦点断言。
- Apple Silicon 独立归档执行 `scripts/smoke-macos-desktop.py`，核对 Wails 窗口、正常退出、隔离 SQLite 和回滚。

## Review Triage Log

- low，已修正。旧版取整后的顶部栏宽度小于等于 760px 时使用双行。220% 缩放下，1757×860 的窗口实测约 760.493 CSS 像素，旧版 Chromium 和 WebKit 均取整为 760 并使用双行。1758×860 两端均切到单行。容器查询使用严格小于 760.5px，两个相邻尺寸进入实际布局断言并通过双引擎专项。
