---
title: 'E13 Apple Silicon 原生更新下载验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e7a5a1c27678b29d92a852e7e7f1365bf17d6257'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

验证 Apple Silicon 桌面设置页从版本检查到原生保存和复制路径的链路。使用隔离发布 API 夹具分别产生保存取消、限流、校验失败与真实写入成功，验证出错后可以重试。

</frozen-after-approval>

## Implementation Notes

隔离验证构建仅替换 Go 更新请求的 HTTP 传输。原生界面、保存对话框、暂存和发布、校验、剪贴板及应用数据库走现有项目代码。

- 两轮 Apple Silicon 隔离验收均完成真实保存和剪贴板检查。最终复跑使用同一个空目录验证失败后的暂存清理，并确认整轮只有三次安装包请求。独立 SHA-256、ZIP 完整性和 SQLite 完整性通过。

## Plan Change Log

## Review Triage Log

## Verification

- 实际操作更新检查与原生保存对话框。取消时不请求安装包。限流和 SHA-256 错误后没有残留文件，能继续重试。
- 成功写入后独立核对测试 ZIP 的 SHA-256。验证系统剪贴板与设置页路径一致，正常退出后 SQLite 完整性为 ok。
