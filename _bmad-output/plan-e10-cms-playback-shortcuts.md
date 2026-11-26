---
title: 'E10 苹果 CMS 播放快捷键'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '9e273b6fe047347213ee9794ee32a9f5961ebf88'
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

苹果 CMS 播放遵循已保存的播放、跳转、换集、全屏及弹幕快捷键。键盘编辑、输入法与面板自身的行为优先。演示端继续沿用现有快捷键设置。

</frozen-after-approval>

## Implementation Notes

提取两类播放器的按键分派。CMS 通过原生媒体状态执行支持的快捷键，并复用可寻址范围和分集判断。

## Plan Change Log

## Review Triage Log

## Verification

- 验证自定义绑定、播放与跳转、换集、全屏和弹幕，以及输入保护。
- 复验演示播放快捷键与设置、目录播放和前端单元。

## Auto Run

- 生产前端构建、122 项 Vitest、197 项播放及互动 Playwright、全屏定稿专项、Go vet 和 Go 全量通过。
- Apple Silicon Wails 通过播放暂停、快进、进出全屏、评论输入和 SQLite 完整性验收。
