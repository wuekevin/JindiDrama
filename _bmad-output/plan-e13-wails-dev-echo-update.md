---
title: 'E13 Wails 开发服务器 Echo 安全更新'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '57cf51e83be8f9f90f3637815325c323acc064c1'
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

把 Wails 开发服务器使用的 Echo 升级到已修复告警 #27 和 #29 的兼容 v4 版本。验证实际开发服务器和生产构建仍能运行，生产依赖范围与打包候选准确记录。

</frozen-after-approval>

## Implementation Notes

只调整 Go 依赖锁定和验收记录。对照已经留存的 .58 依赖告警对照。当前打包批次保持原提交。

## Verification

- 比较 Go 开发和生产构建解析模块及成品构建信息。
- 实际启动隔离开发构建，通过环回请求读取生产构建的页面和前端资源，正常退出。
- Go 全量、vet、模块完整性及生产源码构建。记录两个上游告警的已修版本。

开发构建扫描另外发现 Gorilla WebSocket v1.5.3 的 GO-2026-6278。它和 Echo 都来自 Wails 开发服务器。将 WebSocket 锁到 Go 漏洞数据库标明的首个修复提交，继续用同一开发与生产验收检查。macOS 的直接 Go 编译使用 Wails CLI 提供的 UniformTypeIdentifiers 链接参数。开发服务器会向 HTML 注入桥接脚本，验证原有入口及独立 JavaScript 文件的内容。

## Results

- 开发构建链接 Echo v4.16.6 和 WebSocket 修复提交。升级前后开发标签扫描的可达告警从 3 项降至 0 项，剩余模块级提示来自未导入的 OpenPGP。三种生产架构的依赖列表均不包含此处升级的两个模块。
- Apple Silicon 实测开发 HTTP 首页、入口 JavaScript、Wails 原生 WebView 和正常退出。Go 全量、vet、模块验证和生产构建通过。
- 首次探针在未建成窗口时终止独立进程，暴露了开发构建的启动期退出异常。带隔离应用标识复验正常窗口退出通过，启动期退出另待调查。

## Review Triage Log

- 中等。初次仅对更新后的生产程序做了编译。新增独立 Apple Silicon 生产归档，按项目脚本解包验证 Wails 窗口、正常退出、SQLite 和活动 WAL。
- 低。状态记录添加 Windows x64 与 Intel Mac 的生产依赖日志链接，便于检查各平台范围。
