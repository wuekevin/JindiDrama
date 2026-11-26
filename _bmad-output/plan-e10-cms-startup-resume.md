---
title: 'E10 苹果 CMS 启动续看'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '1c38a8613f0cd72a1ec382e569a3b6419998020d'
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

桌面启动偏好为「上次观看」时，当前苹果 CMS 片源的未看完分集参与恢复。续看以原进度打开并保持暂停。其他片源的记录不可用来播放，失败时保留原记录并回到可用本地启动页。手动进播放页继续显示正常的错误。

</frozen-after-approval>

## Implementation Notes

启动阶段只通过本机 Provider 核对候选归属。分集及播放票据复用播放页现有验证；一次性启动上下文处理失败回退。

## Plan Change Log

## Review Triage Log

- 中 · 片源核验可能晚于手动页面导航。新导航使旧核验失效，补充暂停归属查询后进入历史页面的回归。
- 中 · 取得票据后媒体仍可能加载失败。回退延续到恢复完成，覆盖媒体 404 与恢复完成前的票据过期。

## Verification

- 生产前端构建、Vitest 及设置、原生媒体播放、授权目录与播放 Playwright。

## Auto Run Result

- 基线不能恢复 CMS 书签，修复后 Chromium 和 Apple Silicon 复验成功。原生捕获第 2 集暂停在 7345 毫秒，503 下回到目录且未启动媒体。两个场景均保留 SQLite 历史。
- 生产构建、122 项 Vitest 与 143 项设置、目录和播放回归通过。未触发托管打包。
