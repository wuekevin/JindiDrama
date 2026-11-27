---
title: 'E13 .56 累计本地打包验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e6d89d582e078330d4cd426161b018b48be72465'
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

从当前发行源码完成累计 Windows x64 和 macOS Universal 测试打包。确保两平台使用通过当前回归的同一份前端产物。验证清单、实际安装器内容、归档上 Apple Silicon 运行及 WAL 回滚、生产依赖已知漏洞检查，并记录发布验证范围。

</frozen-after-approval>

## Implementation Notes

包含 .54 后正式发布复核、成品 WAL 冒烟、更新路径保留、快捷键录制焦点和回滚说明。采用独立干净发行源码，Windows 和 macOS 逐项构建。

## Verification

- 前端锁文件安装、生产构建、单测、全量 Chromium 和 WebKit 设置页，Go 全量和 vet。
- Windows 安装器可解包且主程序相同，macOS ZIP 包含两套架构并检查签名。核对 13 项 SHA-256 及归档内实际说明文件。
- 用同一归档验证 Apple Silicon 真实 Wails 就绪、退出及 SQLite；回滚探针验证已提交的活动 WAL 内容。扫描源依赖及三个成品 Go 程序架构。

- 当前 Chromium 全量、WebKit 设置页、前端单测、Go 全量和 vet 均通过。生产前端构建摘要同时覆盖 Windows 和 macOS 构建。
- Windows 起初编译遇到磁盘空间不足，清理旧暂存目录中可重建的依赖后重跑。Mac 两架构编译成功，合并阶段空间不足。保留两套切片并核对新版本构建标识，合并后逐切片比对，沿用同配置的 .54 包结构，重新临时签名。
- 原生 Apple Silicon 窗口、正常退出、SQLite 完整性以及成品进行活动 WAL 回滚通过。存档清单共 13 项。三个程序架构与生产源码依赖扫描未报告已知漏洞。

## Review Triage Log

- 快审确认本地应急组装脚本在签名和归档前清除旧切片，后续失败会妨碍重试。在独立临时脚本中改用单独的组装尝试目录，保留源切片并归置前次应用，Shell 语法检查通过。该脚本未进入项目工作流，这次已经成功并复验的归档无需重新生成。
