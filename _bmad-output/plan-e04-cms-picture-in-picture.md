---
title: 'E04 苹果 CMS 播放页支持画中画'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'd3841288e4cfcf4266a9f8e56499f82fc8873e62'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

按播放页验收要求，在苹果 CMS 播放器中提供画中画操作。复用桌面演示和本地片库使用的标准功能检测、WebKit 支持和状态提示。切集、换源或离开播放页后，旧视频的画中画状态应清除，包括延迟完成的系统请求。播放器缩放后仍能操作该按钮。

</frozen-after-approval>

## Implementation Notes

在已有 CMS 播放视图中加控件，复用 usePictureInPicture。在调用会重置视频的 stopMedia 前让画中画请求失效。

## Verification

- 使用本地视频夹具复现原页面缺少入口，构建后验证标准和 WebKit 路径以及换源、切集和退出。
- 核对默认窗口及 1280×860、220% 的截图，回归 CMS、本地片库和演示播放。

- 原版页面视频就绪后未出现画中画按钮。现在在已有选集旁提供操作，展示状态并保留键盘激活。换源、切集和路由退出走同一清理入口。
- 生产构建、125 项 Vitest、148 项 Chromium 播放回归和两项 WebKit 专项通过。截图覆盖 1448×1086 与 1280×860、220%。用样片配合画中画接口夹具，原生浮窗待验收。

## Review Triage Log

- 已复现旧过渡超时后不再监听已换集的媒体。超时后保留旧元素的完成监听，若其媒体已失效且该元素没有更新的当前请求，则关闭迟到的画中画。WebKit 和 Chromium 通过超时后切集回归。
- 已记录默认与高缩放截图，和既有 CMS 播放布局对照。恢复测试产出的其他跟踪截图，保留专项证据。
