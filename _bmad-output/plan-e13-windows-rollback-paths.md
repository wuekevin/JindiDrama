---
title: 'E13 Windows 回滚的安装路径指引'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '6aac46d97b06e787d18278ce7a7ff680636bdd92'
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

Windows 用户能根据自己的安装方式使用当前程序生成 SQLite 回滚副本。说明覆盖默认系统级和按用户安装，以及便携版或自选目录，并在可执行路径无效时给出可操作的提示。随包副本和仓库说明一致。

</frozen-after-approval>

## Implementation Notes

按 NSIS 的默认路径和按用户路径修正 PowerShell 示例。要求用户选择已知的当前程序路径，以免同时安装多个版本时用错版本。

## Verification

- 提取文档中的 PowerShell 原样解析，用系统级、按用户、便携及找不到程序的路径执行验证。
- 验证生成的 Windows 和 macOS 随包回滚说明与仓库字节一致，运行打包清单相关脚本测试。

- PowerShell 7.7 在 macOS 通过系统级、按用户、便携路径、程序缺失、程序无法启动、目标已经存在的六个场景。实际 CLI 三次生成两个历史断点完好的 SQLite v2。源和已存在副本摘要不变。
- 既有 .54 程序文件在独立目录生成随包文档，两平台文档与源码逐字节相同，13 项 SHA-256 与 13 项 Node 测试通过。Windows 原生执行留待累计验收。

## Review Triage Log

- 已补上确认当前版本程序位置的步骤，适用于各安装方式，特别是同时安装多个版本的情况。
- 按 Windows 与 macOS 的实际代码使用退出菜单标签「退出金帝短剧」。
- 在 STATUS.md 补齐 PowerShell、SQLite 和随包文档验证结果。
- 已区分路径不存在、程序无法启动和 CLI 返回失败。现有文件不可执行时，以 PowerShell 的原始错误补充操作提示。
