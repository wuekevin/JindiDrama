---
title: 'E11 SQLite 备份快照写入隔离'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'fce790aefcd21fdf8eed4280a3decd8c59e5dcd2'
route: full
route_source: auto
risk: high
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 2
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

让桌面数据库备份将一致的紧凑快照写入已经验证权限并固定目录的成品暂存句柄。给定系统临时目录路径不可用或被改指，备份生成不在该路径落盘。给定目标同名或写入中断，不覆盖既有文件或发布半成品。成品保持 SQLite 格式并保留已提交 WAL 内容和现有恢复兼容性。

</frozen-after-approval>

## Code Map

- `internal/app/storage.go` 与各系统暂存实现绑定已打开的备份文件。应用层负责清理、同步及独占发布。
- `internal/storage/sqlite/backup.go` 借用源连接生成一致、紧凑的快照，暂时切换临时存储，然后复原配置。
- `internal/storage/sqlite/backup_vfs.go` 使用固定版本 modernc 的回调 ABI，将虚拟主库映射到原文件句柄。虚拟日志和注册信息在本次备份结束时释放。

## Implementation Notes

SQLite 通过固定版本 modernc 的 VFS 将 VACUUM INTO 写入已验证的成品暂存句柄。VFS 识别一次备份的虚拟主库和日志名，成品写磁盘，回滚日志留在进程内。备份期间 SQLite 源连接的临时存储设为内存，结束后复原。恢复保留现有只读副本处理。

## Plan Change Log

- 96 MiB 同源对照表明完整内存快照的峰值驻留约 245 MiB，原磁盘路径约 19 MiB。改用已有文件句柄驱动磁盘快照。
- VFS 独立原型实测目录替换、关闭及只读句柄，保留原句柄和替代文件隔离。首版 ABI 拷贝在竞态配置下触发 Go checkptr，改用 libc 的切片接口后通过。

## Review Triage Log

- 已修复。纯内存备选在 96 MiB 对照中显著提高峰值驻留，最终成品用磁盘句柄写入。
- 已修复。32 位整数范围检查仅适用于内存日志，主库截断保留 64 位偏移。
- 已补验证。同一源库通过两个路径生成的成品字节一致。25 次竞态回归验证同时备份的两个文件不会混入对方内容。
- 真实 Windows 和 Intel Mac 当前构建待后续集中验收。现有发布目录权限和独占提交约束继续生效。

## Verification

- 备份使用活动 WAL，快照恢复已提交数据。删除的大记录不进入成品。使用旧生成方式与相同数据对照。
- 桌面应用备份在不可用的临时路径下通过，目标冲突和失败没有半成品。运行 Go 全量、vet、竞态及 Windows x64 测试交叉编译。

- 实际跑通 Go 全量、vet、App 与 SQLite 竞态、Windows amd64 的 App 与 SQLite 测试交叉编译和目标 vet。依赖版本未改变。
- 96 MiB 对照数据集含 3072 条数据，完整性检查通过，最终集成方案最大驻留为 20201472 字节。
