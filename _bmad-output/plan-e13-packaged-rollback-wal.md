---
title: 'E13 成品回滚活跃 WAL 数据验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'b160a4771ced0c3e722bed81e8ff6c224aed30b0'
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

把已提交但未检查点的收藏和观看进度加入现有 Windows、macOS 成品回滚验收。用实际成品程序生成 v2，验证完整性、源数据库摘要及逻辑数据。丢失 WAL 记录时检测失败，夹具与应用资料独立。

</frozen-after-approval>

## Implementation Notes

复用两平台共同使用的桌面验收命令。保留原有资料库回滚验证，额外构造合成 WAL 数据并比较回滚件。

## Plan Change Log

## Review Triage Log

- 已修复。两次回滚的每次子进程限时为 20 秒，macOS 外层等待增加到 60 秒。
- 已修复。合成 WAL 测试根位于独立临时目录，Windows 子进程的缓存和 AppData 与原资料库隔离。

## Verification

- 单测通过有效 v2 和只有源主文件两种路径，后者须被发现丢失 WAL 数据。
- 用 .54 ZIP 内真实程序验证新增入口。Go 全量、vet、Windows x64 交叉编译通过。下一批在双平台包工作流原生执行。

- 对 .54 ZIP 复验归档摘要和程序字节后运行新增探针，正例保留 WAL 的收藏及观看数据，反例发现只复制主库造成的丢失。
- Go 全量、vet、竞态、Windows x64 交叉编译、PowerShell 解析及 7 项 macOS 脚本单测通过。Windows 和 Intel Mac 的新增场景待原生运行。
