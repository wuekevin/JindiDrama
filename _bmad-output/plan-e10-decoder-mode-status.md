---
title: 'E10 在设置中显示实际解码方式'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'aa8f97a860780a2c75cef4d02713e3c9c12816b7'
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

设置页提供硬件与软件解码两个可选项，当前实现只写入偏好。用户从控件得不到当前播放器的解码行为。

设置页显示由系统自动选择解码方式。保留旧版偏好数据，真正切换解码器的需求继续在 E10 跟踪。

</frozen-after-approval>

## Implementation Notes

改动集中于设置显示与回归。用隔离发行源码验证默认布局、最小窗口和已有软件解码偏好升级后的显示。

设置页沿用现有次要文字样式，移除已失去用途的单选样式。播放器仍由现有播放链路选择解码器。旧偏好保留在设置的数据结构中，真正切换解码器在 E10 状态记录中继续跟踪。

## Plan Change Log

## Review Triage Log

- 设置页回归应触发实际写回，已更改清晰度并等待持久化结果同时包含旧偏好与新清晰度。
- 验收记录应包含真实结果和视觉对照，已把日志、截图及待实现的切换能力加入 `docs/STATUS.md`。

## Verification

- `npm run build` 通过 TypeScript 和生产构建。
- `npm run test:e2e -- tests/e2e/10-settings.spec.ts` 验证设置页全量。使用已有软件解码偏好验证自动解码状态和偏好数据的保留。对照默认窗口和 220% 窗口截图。
- 原版专项失败于解码状态缺失。修复后 Chromium 设置页 45 项、WebKit 专项和生产构建通过。默认窗口变化的像素范围仅覆盖解码行，220% 截图经目视检查。
