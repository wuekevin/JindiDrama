---
title: 'E13 .63 累计发行候选本地打包'
type: chore
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '592493a9de15d74430c3c05c08559bde33a76cf8'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

从独立发行提交构建累计 Windows x64 和 macOS Universal 测试候选。两套交付文件从归档目录逐项复算 SHA-256，并在 Apple Silicon 运行最终 ZIP 中的应用，确认 SQLite 完整性和回滚。记录真实架构、安装器内容、回归与签名状态。本轮批量完成本地验收，保留 GitHub 和正式签名为独立发布门槛。

</frozen-after-approval>

## Implementation Notes

复用 Windows NSIS、macOS Universal 和两平台清单的已有脚本。锁文件重建并比对内嵌前端文件。使用独立目录保留包和验证日志。

## Verification

- 从干净隔离检出生成两套归档，macOS 两切片最低版本为 13.0。校验最终交付目录和 Windows 安装器内置主程序。
- 原生 Apple Silicon 从最终 ZIP 打开实际窗口、正常退出，检查数据库与回滚。
- 执行 Go 全量与 vet、前端回归、公证及打包脚本测试。按环境记录未运行的平台门槛。

## Results

- 锁文件重建后，两套本地包的 13 项 SHA-256 均与最终目录一致。NSIS 包内 x64 载荷主程序与便携版相同，安装和卸载引导程序均为 x86。macOS 两切片为 arm64 和 x86_64，最低版本均为 13.0。Windows 主程序和 macOS 两切片内嵌的 69 个前端文件、5 个目录均与构建产物匹配。
- Apple Silicon 最终 .63 ZIP 使用隔离 bundle ID 和本机临时签名。应用的正常退出、SQLite 完整性、回滚与更新检查复验通过。用真实 GitHub 查询端点模拟断网恢复，重试后旧错误不再显示，重启后恢复预发布渠道。
- Chromium 406、WebKit 专项 5、Vitest 125、Go 全量、vet 和 App/更新竞态、Python 19 项、Node 18 项及发行历史夹具通过。Go 生产源码和三个成品架构、npm 全量及生产依赖扫描未报告已知漏洞。
- Wails 的配置重排经过 JSON 等价核对后恢复原文件，隔离出包目录的自动生成项已移到证据目录。Playwright 更新的设计截图在验收后恢复，内嵌资产摘要保持一致。
- 首次命令强制 Go 本机小版本而被模块最低版本要求拒绝，使用项目锁定版本后通过。更新页探针曾引用旧验收摘要文件名，另一次复跑发现相同 bundle 的 WKWebView 本地记录保留上轮时间。修正摘要路径并比较失败前后的原状态，复跑通过。仍需 Windows 与 Intel Mac 原生、正式签名与公证、签名版在线升级及授权公网片源联调。

## Review Triage Log

- 低，已修复。分别记录 Windows x64 应用载荷、NSIS x86 安装和卸载引导程序及七件清单。
- 中，已修复。补充 macOS 测试包使用独立 bundle ID 和临时签名。该身份的缓存与正式包独立。
- 低，已修复。记录脚本读取 Git 差异前，将新计划标记为 intent-to-add，确保两个候选分支都包含计划及有效链接。
