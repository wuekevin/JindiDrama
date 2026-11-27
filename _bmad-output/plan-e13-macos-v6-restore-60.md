---
title: 'E13 .60 成品原生恢复 v6 备份'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '6420a25cb91a7b357d0a87a5e03166bb57af77cb'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

在 Apple Silicon 从已归档 .60 ZIP 启动真实桌面程序，通过系统文件选择器把旧 v6 数据库备份恢复到预置的 v10 资料。验证错误或取消保持原记录、重复恢复无重复数据、已有的较新断点不被覆盖。源备份和现有设置、搜索不被恢复修改。

</frozen-after-approval>

## Implementation Notes

使用旧版已留存的 v6 数据库快照与隔离的 v10 测试资料。复用现有 macOS 原生窗口探针。验证后记录证据与平台范围。

## Verification

- macOS 归档清单、归档内主程序字节、临时签名、版本和实际运行架构核对。
- 原生取消文件选择、损坏备份拒绝、v6 导入、重复操作和重启。核对收藏顺序、分集、精确断点、搜索与关键设置。检查源 SHA-256、辅助文件与 SQLite 完整性。

## Results

- .60 的 macOS 清单 6 项通过。解包主程序与归档内字节一致，临时签名、arm64 会话和版本就绪标记均核对。
- 原生设置页和系统选择器完成取消、损坏文件拒绝、v6 导入和重复导入。当前 v10 文件保留 14321 ms 的较新断点、9123 ms 的原有独立条目及原收藏顺序，只补回一项收藏和旧库中精确的 7456 ms 断点。搜索词、主题和启动页保持现值。原 v6 文件及另存所选文件的 SHA-256 相同，无新辅助文件。
- 重启后仍显示三项收藏、三条历史，两个会话正常退出，最终 SQLite integrity_check 通过。App 和 SQLite 的备份恢复专项 Go 测试通过。当前源码的恢复相关路径与归档提交一致。此次原生验收覆盖 Apple Silicon 的本地测试包。

## Review Triage Log

- 已明确读取核对的设置字段为主题、启动页和搜索词。快审复查主工作区的证据文件及链接有效。
