---
title: 'E12 苹果 CMS 设置连接测试与保存共享限流状态'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '7a42c9a3af21aef301be50a3113677bbb2d43e53'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

设置里的苹果 CMS 测试和保存准确反馈连接结果。同一个已规范化的片源返回限流后，这两个入口共用现有 Retry-After 冷却，失败不替换已保存的片源。其他地址可独立测试，冷却到期后可重试。当前启用的同一 CMS 也沿用正常目录请求的限流。连接绑定缺失时不可显示测试成功。

</frozen-after-approval>

## Implementation Notes

采集现有 Provider 的冷却截止时间，设置侧短期保存仍有效的地址。使用已有配置锁串行读取和写入设置，统一测试和保存入口。复用现有固定限流文案与安全地址验证。

## Verification

- 受控 Go 请求复现重复请求，验证规范化 URL、切换地址、限流到期、清除与保存不受失败污染、当前已启用的片源限流。
- 执行 Go 全量和竞态，以及 Chromium 和 WebKit 设置测试。对照错误提示与旧片源保留状态。

## Results

- 原版对同一规范化地址的两次操作均发出上游请求，且只反馈通用连接失败。修复后测试、保存和切换回来重试共用已规范化地址的冷却。其他地址可验证。期满可保存，新地址仍须重启启用。当前已启用的 CMS 与设置共用 Provider 冷却。
- 冷却只留在本次应用会话。设置侧在后续操作时清除过期记录。未到期的记录不会因其他地址进入冷却而提前移除。成功测试不会改写配置。桌面测试方法缺失时报告失败。
- Go 全量、vet、App 和 Provider 竞态、Windows amd64 测试交叉编译，前端类型检查与构建、Vitest 125 项、Chromium 设置 46 项及 WebKit 设置 5 项通过。对照保存状态和提示的原版、修复截图。
- Apple Silicon 隔离应用用真实桌面桥与 Go 的 2 秒冷却，测试后立即保存没有新增上游请求，保留已保存的原地址。期满后成功保存，读取 SQLite 确认规范化后的候选地址。正常退出、SQLite 完整性通过。合成源之外待实网联调。

## Review Triage Log

- 中，已修复。删除提前逐出仍有效地址的策略。34 个候选对照中，快审前代码触发 68 次请求，最终只触发 34 次。
- 中，已修复。公开配置读取使用同一配置锁，已保存地址和启动提示来自同一次读取。
- 低，已修复。清除设置后冷却仍有效。已启用片源由正常目录先收到限流时，设置测试与保存也都遵守同一冷却。
