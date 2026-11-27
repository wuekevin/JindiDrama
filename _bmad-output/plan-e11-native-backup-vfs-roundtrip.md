---
title: 'E11 Apple Silicon 备份恢复原生往返'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e1ac291b8608870a2094fd265bc24c48c55e1900'
route: oneshot
route_source: auto
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

通过实际桌面设置页验收新的 SQLite 备份写入。隔离资料中，已有目标障碍时界面应报告失败，排除后可重试。活动 WAL 中的新记录进入备份。备份保持私有且可通过系统选择器恢复。恢复只补缺失数据，保留较新的本地记录；重启后记录可见。

</frozen-after-approval>

## Implementation Notes

使用独立 Bundle ID、临时签名及合成 SQLite 库。复用已核对的 .53 前端。保留真实 WKWebView 和 Wails 交互，并以 SQLite 查询、源文件摘要和权限对照复核成品。

## Plan Change Log

- 首次原生运行建立 Wails 就绪标记，系统锁屏导致辅助功能无窗口。用仅限隔离构建的 WKWebView 点击脚本继续，真实后端负责备份和恢复。恢复的选中文件从测试夹具返回，原生系统选择器仍待验收。
- SQLite 验证工具在应用退出及辅助文件清理后以只读 immutable 模式检查主库，避免验证过程重建辅助文件。

## Review Triage Log

## Verification

- macOS 设置页实际点击备份、观察失败提示并重试，验证活动 WAL、SQLite 完整性和隐私 ACL。通过系统文件选择器恢复，验证重复导入、取消、正常退出和重启持久化。

- 备份失败和重试、ACL 和活跃 WAL、恢复合并、重复恢复、重启可见及正常退出已验证。未插桩的同源应用从该备份生成完整 v2 并保持源摘要。
- 锁屏下保留系统文件选择器的实际选择和取消缺口。两个应用构建均使用独立标识及合成资料。正式分发和 Windows、Intel Mac 原生验收未涵盖。
