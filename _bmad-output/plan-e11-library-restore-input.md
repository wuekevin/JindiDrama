---
title: 'E11 备份恢复隔离源数据库'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e33010ef3d9ff9b300ea777b4b6e653efe9e8c77'
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

备份恢复应只读用户选择的文件。当 SQLite 以只读模式打开 WAL 数据库时仍可能在源目录创建辅助文件。将 SQLite 的备份读取隔离到现有受保护暂存目录，保留金帝生成的当前版和历史备份，并保留现有稳定 WAL 内容读取能力。读取或完整性验证失败时，保留当前收藏和观看历史。

</frozen-after-approval>

## Implementation Notes

复用回滚读取的数据库文件集快照，以及备份写入已使用的原生暂存目录权限和句柄清理。恢复合并继续使用现有 SQLite 事务。回归测试证明旧版在已关闭的数据库旁创建 WAL 和 SHM。暂存读取需要额外磁盘空间。复制和清理由目录句柄定位，SQLite 打开暂存路径前后校验目录身份。

## Plan Change Log

## Review Triage Log

- medium，已修复。审查指出路径被替换时原实现会把快照写到替代目录。暂存文件和摘要读取现在通过目录句柄定位，打开 SQLite 前校验目录身份。macOS 夹具在复制前替换目录，核实替代目录只有原标记文件，原目录完成清理。
- medium，已修复。审查指出选择当前库后用普通文件描述符读取可能释放 Unix 进程的 SQLite 锁。恢复在暂存前按文件身份拒绝当前库及其别名，回归确认当前数据仍可读取。

## Verification

- 给定稳定的 WAL 数据库，当用户选择时，可读出已提交记录且不会创建或修改源辅助文件。
- 给定金帝生成的旧版或当前备份，当用户恢复时，导入缺少的收藏和历史。
- 给定损坏的备份，当恢复失败时，当前收藏和历史保持完整。
- Go App 和 SQLite 专项、竞态、全量、vet，Windows amd64 交叉编译。

- macOS 专项、Go 全量、vet、App 和 SQLite 竞态及 Windows amd64 交叉编译通过。Windows 原生尚待验收。
