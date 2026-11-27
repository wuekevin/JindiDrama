---
title: 'E11 固定 SQLite 快照读取源文件'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'bef640b9179d99392db2e679bef7a8fb26baaa97'
route: oneshot
route_source: auto
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

备份恢复、回滚与桌面数据库校验要拒绝非普通源文件，避免操作因 Unix 命名管道等待写入方而卡住。生成快照时，复制与摘要验证须来自同一次打开的源文件，检测源文件状态或文件集变化。保留稳定的 SQLite WAL 文件集和现有旧备份兼容性。

</frozen-after-approval>

## Implementation Notes

用只读句柄验证文件种类。Unix 打开时设置非阻塞，再检查句柄指向普通文件。快照继续复核源文件、暂存副本摘要和源路径文件集。桌面验证复用同一快照，并从解析后的主库路径查找 WAL。

## Plan Change Log

## Review Triage Log

- medium，已修复。保留传入路径并在复制、摘要后重新解析，替代链接即使保留原目标也被拒绝。
- medium，已修复。用已打开文件的 Stat 捕获首个身份，避免 Windows 路径 Stat 延后抓取身份。各源在开始复制之前打开和记录；末尾比较身份、大小、修改时间和辅助文件集合。
- low，已处理。增加同大小同时间替换、原文件大小变化、新增 WAL 和符号链接改指测试。

## Verification

- 给定没有写入方的命名管道，桌面数据库复制立即拒绝，暂存目录无输出。
- 给定稳定 WAL 与旧版备份，回滚、恢复和桌面校验仍通过。文件身份或大小变化时拒绝快照。
- Go SQLite 和 App 专项、全量、vet、竞态以及 Windows amd64 SQLite 交叉编译。

- macOS SQLite 和 App 专项、Go 全量、vet、竞态及 Windows amd64 SQLite 交叉编译通过。普通文件 I/O 仍由操作系统负责。
