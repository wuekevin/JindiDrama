---
title: 'E04 Apple Silicon 原生 CMS 画中画验收'
type: chore
ticket: ''
created: '2026-11-27'
status: blocked
blocked_reason: '本机 WKWebView 对隔离样片拒绝画中画。真实浮窗开启与关闭须在可启用的桌面及样片上验证。'
baseline_revision: '2f53254e3b1af7aab47bbf02939e1f341959527a'
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

用 Apple Silicon 的 Wails 客户端实际播放隔离的 CMS 样片，通过生产界面控件请求真实 WebKit 画中画并检查离页清理。记录系统模式和可观察的浮窗证据，区分合成片源、浏览器模拟和原生行为。

</frozen-after-approval>

## Implementation Notes

沿用项目原生 CMS 验收的受控目录与样片。只在临时构建注入合成传输和状态观察脚本，画中画实现、窗口和浏览器接口均为实际运行。

## Verification

- 校验生产前端文件、临时原生构建和隔离配置。
- 真实 WebKit 解码样片，并通过 macOS 辅助功能驱动界面，观测画中画开启、退出和数据库关闭。

- 样片通过实际 Go 转发达到可播放状态。等待原生可访问树，带按压状态的按钮在 macOS 暴露为 AXCheckBox。真实点击有用户激活，系统调用返回 NotSupportedError 后应用保持内联播放，返回详情清理视频。
- 另用独立 Swift WKWebView 加载相同样片，获得相同拒绝。已验证本机的原生降级。没有观测到实际系统浮窗。

## Review Triage Log

- 正向系统浮窗验证留待支持环境。现有结果只证明本机样片被系统拒绝、应用内联继续和独立对照一致。
- 重跑补采请求被拒后及返回详情后的 DOM 画中画状态、WebKit 模式和窗口枚举。
- 独立 Go 测试验证存储生命周期关闭；实际应用正常退出，无运行会话标记及 SQLite 辅助文件，隔离数据库完整性通过。
