---
title: 'E12 苹果 CMS 分集本机弹幕'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '7c000c38d74a0c00dee78b8f29bdd410ace2e3a5'
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

苹果 CMS 播放支持按片源、剧目和分集保存本机弹幕。绑定实际媒体时间轴，沿用既有轨道、偏好、补写和关闭窗口保存。弹幕正文留在本机。

</frozen-after-approval>

## Implementation Notes

抽出两类播放页共享的弹幕加载、发送和补写。苹果 CMS 复用本机评论的片源身份验证。

## Plan Change Log

## Review Triage Log

## Verification

- 用 SQLite 验证离线写入、幂等、换集隔离和重启。
- 验证实际视频时间轴、弹幕记录、换集和刷新，并回归演示弹幕和目录播放。

## Auto Run Result

- 复用演示端加载、发送和补写，CMS 只从本机记录生成弹幕轨道。保存使用当前配置片源和分集键。
- 122 项 Vitest、94 项浏览器回归、Go vet、Go 全量及互动竞态通过。Apple Silicon Wails 在片源故障时保存实际时点，通过重启与分集隔离，SQLite 完整。
- CMS 面板沿用本机评论的下方布局，复用原有弹幕控件。无远端推送或打包。
