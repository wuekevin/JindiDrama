---
title: '演示和本地授权播放在窗口隐藏时保存进度'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: f826a91676cb07949515f951ea2405dad959f4cf
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

收起窗口时，演示和本地授权播放应记录刚播放到的位置，且不改变当下播放状态。演示播放在页面离开时同样保存进度，避免短于定期记录间隔时丢失位置。

</frozen-after-approval>

## Implementation Notes

复用现有保存函数、去重和恢复保护。监听桌面隐藏或页面离开事件，在页面销毁时解除监听。关闭流程继续使用已有握手。

## Verification

- 用真实本机测试视频在定期记录前发出生命周期事件，验证播放位置、播放状态和不产生关闭回执。
- 运行相关播放器、授权片库、苹果 CMS 浏览器套件及前端构建和单元测试。


## 实际验收

- 演示视频在窗口隐藏和页面离开时、本地片库视频在窗口隐藏时，旧构建均漏存刚开始的进度。追加顶栏最小化按钮回归后，确认该入口也曾漏存。修复后四条用例均保存当前秒数并保持播放。
- 最终前端类型检查、生产构建、122 项 Vitest 和设置、演示、授权片库、CMS 共 164 项 Playwright 通过。Go vet 与全量通过。原生系统窗口操作留待集中验收。
