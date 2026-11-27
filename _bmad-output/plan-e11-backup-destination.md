---
title: 'E11 将 SQLite 备份固定到所选目录'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'dea29eec072e6f2b25c8943a550bc9d2cf5acf77'
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

SQLite 备份的暂存路径来自备份目录。若该目录在生成快照时改向，旧代码可能把替代目录中的文件当成备份发布。

在启动 SQLite 快照生成前打开原备份目录。快照在系统私有临时目录生成，再从文件描述符复制到已固定的目标，继续独占发布。目录发生变化须提示，替代目录的文件保持原样。保留现有备份一致性、禁止覆盖和清理规则。

</frozen-after-approval>

## Implementation Notes

复用媒体保存已验证的目录固定与独占发布。SQLite 快照暂存在系统临时目录，备份期间需要额外的快照文件空间。

现有备份接口继续返回成功保存的路径。设置页仅展示明确允许的目录错误。原备份目录改向时提示检查数据目录，已保存到原目录时提示查找位置。

## Plan Change Log

## Review Triage Log

- 快照复用受保护的 Windows DACL 和 macOS 私有目录及清 ACL。临时文件通过已打开的目录清理。
- Windows 目录链接夹具仅在系统报告缺少链接权限时跳过。macOS 夹具验证了实际继承 ACL 被清除和替代目录文件得到保留。

## Verification

- 用目录改向和替代文件复现问题，对照修复后的目标错误与替代内容。
- 运行备份、恢复、同名冲突、部分写入、Go 全量及竞态。Windows amd64 测试程序交叉编译。
- 原版夹具改向后误报成功。修复后的 macOS 目录、ACL、SQLite、Go 全量与竞态，以及 Chromium 设置页 45 项均通过，Windows amd64 测试交叉编译通过。
