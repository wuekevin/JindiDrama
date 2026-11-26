---
title: 'E10 苹果 CMS 播放高缩放'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '324dd0b7ca09d54d1633e55fe51141134c41e0f5'
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

苹果 CMS 播放应在最小桌面窗口和 220% 用户缩放下保持可操作。全屏保持竖屏视频比例，退出后可操作选集、评论与弹幕。

</frozen-after-approval>

## Implementation Notes

用生产前端和本机可播放的 CMS 夹具检查窗口内与全屏布局，在测试中验证控件可操作和视频边界。

220% 的最小窗口中，修复前视频从 65vh 被二次放大到 1023 像素。将普通播放的高度上限按生效缩放折算；全屏样式继续按全屏容器布局。

## Plan Change Log

## Review Triage Log

- 低 · 测试夹具缺少互动写入。补充评论和弹幕回执，并以实际点击、请求快照和列表记录验证。
- 低 · 全屏此前只检查外框。新增竖屏素材宽高比及 contain 断言，并复核截图。
- 低 · 补录构建、回归、原生验收及截图，在 STATUS.md 完成仓库规定的记录。

## Verification

- 运行前端生产构建、CMS 播放与演示布局相关 Playwright，保存截图并对照播放设计和既有 CMS 叠加截图。

## Auto Run Result

- 复现 220% 时高 1023 像素。修复后 Chromium 和 Apple Silicon 在 860 像素高的视口内均通过。
- 通过生产前端构建、122 项 Vitest、82 项播放 Playwright 和快速评审后的定稿专项。
- 对照播放设计稿检查画面和操作控件，保存普通播放、全屏、评论和弹幕截图。原生复验使用本机合成片源。
