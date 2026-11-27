---
title: 'E13 .58 累计本地打包验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '5a9181690e9356ba3003d49b5c147ec606a775b2'
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

从当前独立发行源码为 Windows x64 和 macOS Universal 生成同一批 .58 测试包，纳入 CMS 播放画中画、手动切源和 .56 之后的发布复验。核对两平台成品中的同一份生产前端和各自的 SHA-256 清单，并验证 Apple Silicon 上直接从归档启动。

</frozen-after-approval>

## Implementation Notes

隔离源码与生成目录，使用仓库现有 Wails、打包说明和清单流程。Windows 和 macOS 构建运行在 Apple Silicon 主机。正式签名和公网运行仍分别记录。

## Verification

- 前端锁文件安装、生产构建、前端单测、Chromium 全量与 WebKit 切源专项。Go 全量、vet、发行脚本和生产依赖检查。
- Windows NSIS 主程序、便携版和 WebView2 引导程序比对。macOS 解包、签名、arm64/x86_64 和系统版本核对。
- 二进制内的前端资源摘要与已测生产构建相符。13 项 SHA-256 使用项目脚本和独立校验工具检查。
- Apple Silicon 从最终归档启动、退出，验证 SQLite 和回滚探针。记录包大小和各平台验收范围。

## Results

- Windows x64 和 macOS Universal 使用相同前端，Windows 主程序与 macOS 两套切片分别匹配全部 69 个嵌入资源。Windows 安装器中的程序和 WebView2 引导程序比对通过。macOS 最低版本均为 13.0。
- Chromium 400 项、WebKit 4 项、Vitest 125 项、发行脚本 17 项、Go 全量和 vet 通过。源码、三架构成品和 npm 生产依赖扫描未报告已知漏洞。
- 归档后 13 项 SHA-256 匹配。Apple Silicon 从最终 ZIP 解包启动，通过实际窗口、正常退出、隔离 SQLite 和活动 WAL 回滚。
- 本地 Windows 与 macOS 为测试成品。Intel Mac 和 Windows 原生、正式签名、公证、在线升级和授权公网联调仍待验收。

## Review Triage Log

- 中等。初次仅从可执行文件中匹配生产文件的字节。复验从 PE 和两套 Mach-O 的 Go embed.FS 表读取全部路径和内容，对照路径集合与逐文件 SHA-256。故意改动测试副本中的嵌入路径时，复验按预期拒绝。
