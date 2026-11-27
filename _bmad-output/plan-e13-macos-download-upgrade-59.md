---
title: 'E13 Apple Silicon 下载 .59 成品并验证升级'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '88ab1f441c0e8f69612db63e1f60d84b6521e9ca'
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

用 Apple Silicon 的旧版原生设置页，通过预发布更新列表发现 .59，并在系统保存对话框下载、校验实际归档 ZIP。使用下载的 ZIP 替换隔离安装目录，启动原始新版程序，核对收藏、历史及 SQLite 完整性。正式线上发布独立验收。

</frozen-after-approval>

## Implementation Notes

旧版测试构建基于 .52 独立发行提交。测试仅替换请求传输，固定系统对话框的起始目录。下载的目标是已归档的 .59 成品。系统签名验证与资料库检查在隔离目录完成。

## Verification

- 核对旧版与 .59 归档清单，老版测试构建的嵌入前端与 .52 原归档逐文件一致。
- 实际执行版本检查、系统保存、成功提示、独立 SHA-256 核对、原下载解包和临时签名验证。
- 用相同资料目录启动下载的新程序，确认版本与两项收藏、两条历史。核对精确断点、设置、搜索、正常退出以及 SQLite 完整性。

## Results

- `.52` GitHub 归档和 `.59` 本地归档的 macOS 清单各 6 项匹配。`.52` 原归档与旧版测试构建的 69 个内嵌文件逐路径比对一致。
- `.52` 旧版原生设置通过合成预发布元数据发现 `.59`，用实际 macOS 保存对话框写出 ZIP。页面显示校验成功。下载 ZIP 与归档逐字节一致，独立 SHA-256、ZIP 完整性和 macOS 隔离标记均通过。首次链接空间不足，重跑成功。最初的保存路径检查按字符串比较，改为判断文件的实际路径后完整复跑通过。
- 直接运行从下载 ZIP 解出的原程序，在同一隔离安装路径和预置的 SQLite v10 资料路径启动并显示 `.59`。两次正常退出，最终 `PRAGMA integrity_check` 通过，2 项收藏、2 条历史、精确断点、搜索和关键设置保留。执行使用 Apple Silicon，下载归档内为临时签名，正式签名、系统分发策略和 GitHub 线上下载仍待验收。

## Review Triage Log

- 已补 `docs/STATUS.md` 状态总览和详细证据链接。项目主工作区保留原生、归档及数据库读回记录。
- 已明确使用预置 SQLite v10。两次关闭后的快速核对之外，最终数据库另通过完整 `PRAGMA integrity_check`。执行方式记录为直接启动下载归档中的主程序。
