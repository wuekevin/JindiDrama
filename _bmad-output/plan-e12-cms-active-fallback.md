---
title: 'E12 苹果 CMS 播放中断后继续播放备用源'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '4177fc72360af4f9e4bb6609e3d78c57eb5e1c25'
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

正在播放的苹果 CMS 媒体中断时，在同一票据的备用源恢复断点后尝试继续播放。暂停或尚未播放的分集继续等待用户。浏览器拒绝接播时清楚提示手动播放。

</frozen-after-approval>

## Implementation Notes

沿用现有的断点恢复与接播边界。区分自动下一集和当前分集切源，以保留对应提示。

## Plan Change Log

## Review Triage Log

## Verification

- Chromium 比对播放中、暂停和浏览器拒绝三种情况，检查断点、静音和取票次数。
- 运行生产构建、单元和目录播放回归。

## Auto Run Result

- 正在播放的分集切换备用源后恢复位置、保持静音并尝试接播。暂停与浏览器拒绝时保留手动入口。
- 生产前端构建、122 项 Vitest、66 项目录和播放回归通过。新回归覆盖播放中、暂停、浏览器拒绝和手动恢复。
- 以自制样片验证 Chromium，未触发 GitHub 打包。本改动的原生路径仍待联调。
