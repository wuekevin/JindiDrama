---
title: 'E12 Apple Silicon 原生片源回退与下架验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '098e1b5b8a7cb482282c1cfefb718fab08a1fafd'
route: oneshot
route_source: pinned
risk: medium
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

验证 .50 在 Apple Silicon 上使用真实 WKWebView 播放。上游直链返回标记为视频的 HTML 错误页时，通过正常 Go 转发切到备用源。上游标记剧目下架后，目录与旧链接应遵守下架状态。

</frozen-after-approval>

## Implementation Notes

测试构建与 .50 使用同一份前端。通过构建覆盖提供合成目录和合法自制样片。系统辅助功能在当前桌面未返回可操作窗口，使用 Wails 原生窗口执行隔离 JavaScript 完成浏览器交互。验证在独立应用标识和数据目录中运行。

- 原生进度 1.33 秒，分辨率 240×426。请求日志确认先失败后成功的两个实际源。系统打开已下架旧链接显示未找到，首页仅保留仍上架的其他剧目。两套独立 SQLite 完整性通过。

## Plan Change Log

## Review Triage Log

## Verification

- Apple Silicon 从首页进入实际播放，确认错误主源与备用源的请求，并验证媒体解码和播放进度。
- 切换合成片源上架状态，使用原生系统深链重新打开目标。核对目录隐藏、旧链接错误态、没有新的媒体请求。
- 两次应用启动使用隔离 SQLite，正常退出后核对数据库完整性。
