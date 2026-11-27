---
title: 'E01 首页续看可重试片源核验'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '80f646e20680a8482872dfe3f15313023abf3fec'
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

首页明确呈现继续观看中暂时核验失败的剧目，并可原页针对失败项重试。已有可继续观看的项目不受影响。确认下架的响应保留不可用状态。恢复后续看使用原有断点。

</frozen-after-approval>

## Implementation Notes

复用收藏与历史的失败状态和逐条重试，沿用三条共享并发槽。首页续看提供一个针对失败项的入口，保留前往历史的入口。键盘触发后若按钮消失，焦点进入同区历史链接。

## Verification

- Chromium 和 WebKit 覆盖目录可用、暂时核验失败、明确下架和本地续看并存。验证只重试失败项、重试期间状态、焦点和断点。
- 运行生产构建、Vitest、首页及 Provider 播放回归。复核首页截图。

## Results

- 原版 WebKit 在目录与本地续看卡片可用时复现 CMS 失败项没有首页重试入口。修复后可见失败剧目标题或 CMS 编号及状态。选择重试只重新请求失败记录，确认下架的记录不加入队列。原有本地续看保持可用。
- 前端生产构建、Vitest 125 项、Chromium 首页和 Provider 播放 94 项及 WebKit 首页专项 7 项通过。默认视口截图核对继续观看标题、两个操作及失败剧目状态。WebKit 验证键盘触发后重试成功和确认下架均保留至历史入口的焦点。
- Apple Silicon 原生 Wails 隔离构建用合成 CMS 响应与真实 MP4 样片，从首页恢复后打开暂停在原 6000 ms 断点的分集。正常退出、SQLite 完整性和原记录通过核对。

## Review Triage Log

- 中，已补。已有可续看卡片时，独立列出失败剧目和状态。片名可从当前目录取得时显示标题，否则显示本地 CMS 编号。
- 中，已补。主目录为空或报错时保留当前片源的续看区，确认下架也可把焦点移到历史入口。新增专项覆盖。快审复核两项均通过。
