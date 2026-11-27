---
title: 'E13 .54 原包窗口与系统选择器'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '87b8a924db5bfe1c5e079a117f696b5d5cbbed99'
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

补齐锁屏时无法验收的 .54 原包可见窗口及 Apple Silicon 系统备份选择器。验证备份重试、实际选择与取消、只补缺失项和重启持久化。通过同一个归档成品检查公开稳定更新频道并核对无正式版本时的状态。

</frozen-after-approval>

## Implementation Notes

使用既有 .54 ZIP 和独立资料目录。通过 macOS 辅助功能驱动实际按钮与系统选择器。备份源为合成数据，SQLite 查询和摘要独立复核。

## Plan Change Log

- 第一次原包运行发现默认启动续播会更新合成历史。验收夹具指定推荐首页，避免正常续播影响恢复数据对账。

## Review Triage Log

## Verification

- 核对程序字节、临时签名和归档清单。原生可见窗口、设置页备份、ACL、仅 WAL 的已提交记录、实际系统选择器及重启。
- 实际公开稳定频道检查和应用退出。

- .54 原归档程序可见窗口与真实系统选择器的选择和取消均通过。备份与恢复、SQLite 完整性及源文件摘要通过。原包实际查询稳定频道并显示暂无正式发布版本。
- 本轮为 macOS Apple Silicon 原生验收。Windows、Intel Mac 与正式线上更新仍待验证。
