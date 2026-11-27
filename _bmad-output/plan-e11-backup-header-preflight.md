---
title: 'E11 SQLite 备份文件头预检'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '749abede0558b5ee400efcb60ac799c9df16ae40'
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

给定选中的文件没有 SQLite 辅助文件，若它为空、头部截断或明显不是 SQLite，小范围读取后直接拒绝，避免耗时复制及消耗临时空间。旧版备份与活动 WAL、回滚日志文件仍走现有独立暂存和 SQLite 验证路径。已有收藏、历史和输入文件保持原样。

</frozen-after-approval>

## Implementation Notes

在共享快照复制入口中用已经打开的源句柄读取固定长度的头部。交给 SQLite 负责页面、模式和完整性检查。不增加备份大小上限。

## Plan Change Log

## Review Triage Log

- 已补验证。空文件、截断头和 4 MiB 非 SQLite 文件的内容逐字节比对。附加 WAL 或回滚日志的截断输入完整复制，后续交给 SQLite 检查。

## Verification

- 旧版与修复版分别运行稀疏伪装备份夹具，证明拷贝前拒绝并且暂存目录为空。
- 运行 SQLite 备份、恢复、回滚以及 Go 全量、vet、竞态与 Windows amd64 交叉编译。

- 旧版三个回归均确认创建了暂存文件。修复后专项、Go 全量、vet、App 与 SQLite 竞态及 Windows amd64 交叉编译通过。
- 同源回滚工具拒绝伪装备份并保持原文件摘要。从实际桌面生成的备份仍可生成完整 v2，源摘要一致。
- 第一次跑全量时共享 Go 缓存缺少依赖编译文件。改用隔离缓存后完成全部验证。
