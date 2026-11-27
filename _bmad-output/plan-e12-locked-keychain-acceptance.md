---
title: 'E12 验收锁定的 macOS 系统钥匙串'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'de80459a93929e462b568f86c0658d86ccfd498b'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

在 macOS 真实文件型钥匙串锁定时验收现有凭据库。用临时钥匙串及合成值检查读取、写入、删除、交互状态恢复和系统配置。

</frozen-after-approval>

## Implementation Notes

临时检出为 SecItem 查询指定测试钥匙串和单独的查询列表。第一次尝试仅指定了用于写入的钥匙串，读回验证失败。添加独立查询列表后完成实测。应用生产源码未变更。

## Plan Change Log

## Review Triage Log

## Verification

- 对比 .52 实际打包源码和当前发行源码的凭据库文件，无差异。
- Apple Silicon 真实锁定后读取和写入失败。删除成功且解锁后可观察。默认钥匙串及搜索列表的快照完全相同。
- 恢复原始代码后以 macOS 13.0 部署目标运行竞态测试。无 CGO 和 Windows x64 交叉编译通过。
