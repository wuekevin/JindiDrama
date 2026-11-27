---
title: 'E10 设置页快捷键录制焦点'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'b774d32548dd5ff14fefa544749bad8cfdff317e'
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

用户点击设置页的快捷键录制按钮后可直接输入新快捷键或按 Esc 取消，在 WebKit 中也获得相同操作结果。使用各浏览器实际用于遍历按钮的按键核验设置页的键盘操作。

</frozen-after-approval>

## Implementation Notes

录制按钮激活后显式获取焦点，保持滚动位置。原有设置页按键处理因此能够接到录制按键。三个原有遍历测试沿用该项目已采用的 macOS WebKit Option+Tab，保留真实按键和焦点断言。

## Review Triage Log

- 快审指出缺少验收状态。本次在 STATUS.md 补充原版对照、两套完整设置页结果和原生验收的范围。

## Verification

- 先用原产物运行新增焦点断言，区分录制缺陷与 Tab 遍历差异。
- 重建前端并运行 Chromium、WebKit 设置页和前端单元测试。

- 原产物配合更正后的测试按键，三个按钮遍历用例通过。点击后焦点和 Esc 的录制场景仍失败。
- 修复后的 Chromium 和 WebKit 设置页各 45 项，Vitest 125 项，生产构建、Go 全量及 vet 通过。
- Apple Silicon 的源版和本地修复构建均可通过辅助功能操作令录制按钮取得焦点，也均正常退出。操作系统合成键盘输入未验证实际录入结果。按键录入和 Esc 的已验证范围为 Chromium 与 Playwright WebKit。
