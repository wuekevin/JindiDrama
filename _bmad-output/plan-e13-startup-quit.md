---
title: 'E13 启动期间退出保护'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '5d1c34def3a0c933eae822155a80e8144fcd4969'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

启动时收到退出请求，应用应在原生窗口和本地存储初始化完成后安全退出。正常窗口关闭继续遵循已有写入确认和平台偏好。

</frozen-after-approval>

## Implementation Notes

Wails 会在进入原生窗口创建前接收系统退出信号。此时关闭钩子发送事件和直接返回允许关闭两种路径均可能访问空窗口。等待 Wails 的启动回调，再重新提交此前的退出请求。测试使用临时 Go modfile 指向 Wails 副本，并通过 overlay 暂停窗口构造。生产源码及模块缓存原文件保持原样。

## Verification

- 对同一启动窗口暂停脚本，运行修改前和修改后的 Apple Silicon 应用，启动期间发送系统终止信号并验证退出。
- 核对正常关闭的桌面窗口、SQLite 完整性与活动 WAL 回滚。运行 Go 测试、vet 和 Windows 交叉编译。

## Results

- 在同一测试夹具中，原版在暂停创建窗口时收到 SIGTERM 后崩溃。修复版在窗口仍未创建时保留退出请求，继续启动后正常退出，SQLite 和会话标记检查通过。
- Apple Silicon 生产归档正常启动、关闭、SQLite 检查和活动 WAL 回滚通过。Go 全量和 vet、前端关窗专项 2 项及 Windows x64 生产交叉编译通过。

## Review Triage Log

- 中等。Windows 的普通关闭偏好会介入启动期退出重放。改用应用既有显式退出函数。Go 测试与 Windows 交叉编译通过，Windows 实机仍待复验。
- 中等。早到关闭钩子可能持有未完成的 Wails context。启动状态变更前发布启动回调的完整 context，关闭只在状态就绪后使用它。
- 中等。Wails 启动前已有 SQLite 初始化和会话标记。增加进程启动期的 SIGINT 和 SIGTERM 接收，在 Wails 接管且初始化完成后移交。新增原生夹具覆盖 Wails 注册信号之前和创建窗口之前，两处均证明修复版正常退出。
