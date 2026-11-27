---
title: 'E13 Apple Silicon 旧包本地升级与回滚'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'b999bfa70696689038ad98b93902abf709bf875d'
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

从已校验的早期 macOS 安装包启动旧客户端并建立隔离测试资料。原位置换成本地当前候选，检查新版本打开旧资料后迁移和正常退出。使用新程序生成该旧版可读取的回滚副本，换回旧原包检查收藏与历史，再恢复新版资料核对完整性。记录本地替换范围和正式交付尚待验收的环节。

</frozen-after-approval>

## Implementation Notes

使用用户资料目录之外的独立应用目录和 SQLite。应用与资料替换均在退出后进行。

## Verification

- 核对原归档 SHA-256、两代程序身份与签名、应用窗口和数据库实际迁移。
- 校验回滚工具、实际旧客户端与还原后的当前客户端。

- 旧归档为 .1，原生启动创建 v6，退出后写入合成的收藏、断点、搜索记录与设置。两个应用使用同一隔离安装路径和资料目录。旧包、新构建二进制分别核验，当前应用为 Apple Silicon 临时签名构建。
- 四次可见界面核对版本、两项收藏与两条历史，均正常退出。SQLite 检查通过旧 v6、新 v10、回滚 v6 和恢复后 v10。转换保留源摘要，回滚前后保留两份 v6 库及当前数据库记录。先确认会话已结束，关闭的合成库若残留侧车文件，完成检查点后仅移出空 WAL、对应 SHM。
- 本轮使用本地替换，没有运行正式在线下载、系统分发策略验签或 Intel Mac 安装。

## Review Triage Log

- 快审确认原脚本在只读 SQLite 核验前未证实没有新侧车。重新用空资料目录执行全部往返，在回滚命令前后检查源与目标的 WAL、SHM、journal，并在打开 v6 副本前再次确认。通过。
- 快审指出状态头部可将本地 .55 与双平台打包候选 .54 混同。更新头部范围并在本节记录 .55 及精确编译源码。
