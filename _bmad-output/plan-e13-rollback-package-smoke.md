---
title: 'E13 双平台成品回滚冒烟'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '8ad94030c5d8d435f3e47ba482eee52a51dafd7b'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

Windows 安装版与便携版、Apple Silicon 与 Intel 的成品冒烟都应执行回滚命令。当前桌面进程正常退出后，用其隔离数据库生成 v2 副本，验证旧版所需的 SQLite 结构、完整性、原文件及无桌面启动副作用。验收流程和正式发行仍按既有批次运行。

</frozen-after-approval>

## Implementation Notes

复用桌面数据库探针的迁移与完整性核验。macOS、Windows 共用回滚探针，原生验证用本地隔离归档。

## Plan Change Log

## Review Triage Log

- 中 · 独立核验暴露 SQLite 只读打开原库仍可创建持久辅助文件。始终复制并验证原库文件组，在同一份私有副本执行迁移，原路径交由用户保留。
- 中 · 预期结构曾复用降级链路，无法识别共同缺陷。改为从空库升级到 v2，并拒绝多余对象，验证缺索引和多余对象的夹具均失败。
- 低 · Windows 启动配置另外指定新 WebView2 路径。macOS 归档冒烟检查不会进入新正常启动配置，沿用同一 bundle ID。
- 低 · 活跃写入期间文件组变化会中止回滚。回滚说明记录先退出并重试。迁移复用同一份临时数据库，避免同时创建两份完整副本。

## Verification

- Go 和 Python 测试、Windows 探针交叉编译、Apple Silicon 解包后的真实命令正反验收。

## Auto Run Result

- Apple Silicon 从本地测试归档验证正常窗口、退出和成品回滚。旧代码在新增检查下失败，修复后通过。
- Go vet、Go 全量、SQLite 与探针竞态和 13 项 macOS 工具测试通过。Windows GUI 和探针编译通过。
- Windows 安装版与便携版和 Intel Mac 的新增回滚步骤等待下一批原生打包。
