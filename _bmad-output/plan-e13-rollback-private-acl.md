---
title: 'E13 回滚副本继承私有访问权限'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '3be05ef4e757782d1553010f77e9f4957954c044'
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

回滚把 SQLite 用户数据写到输出目录旁的暂存目录，并用独占硬链接发布。先确保暂存目录权限为私有，避免共享父目录中的 ACL 传给数据库副本和 SQLite 辅助文件。保留回滚目标版本、硬链接发布与现有文件完整性。

</frozen-after-approval>

## Implementation Notes

使用应用已有的 macOS ACL 清理和 Windows 私有 DACL 定义。在建立访问保护前暂存目录保持空白。只对已核对身份的目录应用权限。清理通过原句柄处理已知暂存文件，移除原目录时仅执行空目录删除。

## Plan Change Log

## Review Triage Log

- medium，已处理已有内容保护。暂存保护前后检查路径身份和空目录，发现已有文件时不删除同名内容。macOS 同时核对目录所有者。初次创建到首次打开之间的目录身份，以及降级和发布时的路径重解析仍需后续句柄级发布处理。
- medium，已处理数据清理。通过原句柄删除已知暂存文件，外部路径只尝试空目录删除。macOS 夹具验证目录被替换时原副本清理且替代数据保留。空目录身份核对与系统删除不构成原子操作。
- low，已处理。macOS 检查实际继承 ACL、暂存目录、辅助文件样本和最终硬链接。Windows 用例检查受保护的 DACL、辅助文件与最终副本，已有交叉编译证据。
- low，已处理。验证和边界记录在 STATUS.md。

## Verification

- 给定具备可继承访问 ACL 的 macOS 父目录，旧版可复现回滚副本继承权限，修复后生成的回滚数据库和暂存权限均私有，数据库完整性通过。
- Windows 原生测试覆盖保护后的 DACL 以及具备继承规则的父目录，编译并记录实际验证范围。
- Go 专项、全量、vet、竞态和 Windows amd64 SQLite 交叉编译。

- macOS 原生继承 ACL 回归、Go 全量、vet、App 和 SQLite 竞态及 Windows amd64 两套测试交叉编译通过。macOS 无 CGO 模式明确拒绝继续，验证没有留下文件。
