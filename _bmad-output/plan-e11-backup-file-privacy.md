---
title: 'E11 Windows 数据库备份权限'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '7d9ad40fc416e69e7eed8a3faf28a70d67d1936e'
route: oneshot
route_source: auto
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

创建数据库备份时保护暂存文件及最终 SQLite 文件。给定可继承读取权限的备份目录，通过现有应用备份入口创建的文件应由当前用户和 Windows SYSTEM 访问。给定文件系统无法保留私有权限，应用应在写入备份内容前返回失败。保持备份独占发布、现有恢复读取及媒体导出行为。

</frozen-after-approval>

## Implementation Notes

修改范围为备份发布暂存类型和 Windows 私有文件描述符。复用现有目录句柄、受保护目录描述符、独占改名和清理，文件创建时加受保护 DACL。Windows 测试使用隔离目录与可继承 ACL 对照文件。

备份发布和备份生成使用专用暂存类型。在 Windows 上核对实际目录和文件的保护标志及完整 ACE，恢复读取保留原有暂存类型。

## Plan Change Log

## Review Triage Log

- 已修复。备份新增权限校验曾影响还原读取，共用入口现按操作选择暂存类型。
- 已修复。初版只检查 DACL 标志，现检查实际 ACE，用独立夹具构造预期权限，检查对照文件中的读取授权和成品权限。
- 已修复。网络卷可能无法回答卷信息查询，现对实际对象验证权限，保留支持 ACL 的网络卷路径。
- 延后独立修复。SQLite 快照的路径参数仍依赖暂存目录的路径解析，既有目录身份比对无法覆盖 SQLite 内部写入窗口。

## Verification

- 对照目录上的可继承权限和备份成品的受保护权限，核对成品记录、文件数以及暂存清理。
- 本机运行备份恢复专项、Go 全量、vet 和关键竞态。对 Windows amd64 编译 App 测试并如实记录执行环境。

- macOS 专项、Go 全量、vet 及 App 和 SQLite 竞态已通过。首次全量测试的隔离源码使用前端符号链接，Go embed 拒绝链接。改用经摘要复验的相同前端文件后全量通过。
- Windows amd64 权限测试已交叉编译，目标 vet 已通过。实际 NTFS、FAT 及网络卷验收待运行。
