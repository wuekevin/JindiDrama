---
title: 'E11 Apple Silicon 原生备份恢复验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '2a46e7b46f86728ccc1a455bd3a00f20ed725f34'
route: oneshot
route_source: auto
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

在真实 Apple Silicon 桌面验证已实现的备份恢复。由设置页打开 macOS 系统文件选择器，验收正常导入、重复导入、损坏或越界数据、取消选择和重启持久化。在独立数据目录和 WebView 配置中运行。

</frozen-after-approval>

## Implementation Notes

使用当前发行源码和前端生产构建，用 Wails 生成 arm64 临时签名测试应用。启动页选择推荐首页以排除正常播放产生的进度写入。用指定进程的辅助功能树定位 WebView 按钮、系统选择器和反馈。测试采用合成记录。

## Plan Change Log

## Review Triage Log

## Verification

- 给定有效备份，通过系统选择器恢复应只增加缺失记录，且原有一条播放记录应逐字段保持不变。重复选择时提示无需补回。
- 给定损坏或时间戳越界的备份，应返回失败，当前库保持原值。取消时关闭选择器，当前库保持原值。
- 检查三份输入的 SHA-256 及文件集合。正常退出再启动，收藏和历史页显示两条预期记录，退出后的 SQLite 完整性及会话标记检查通过。
- macOS 26.6.2 arm64 原生全部通过。Windows 和 Intel Mac 的当前源码原生功能验收仍待完成。
