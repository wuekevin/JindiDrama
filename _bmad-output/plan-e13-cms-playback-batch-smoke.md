---
title: 'E13 CMS 播放批次双平台本地验收'
type: chore
ticket: ''
created: '2026-11-26'
status: built
baseline_revision: '76ae67981629490c597545124244ec551118d823'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
followup_review_recommended: false
context: []
warnings: []
deferred: []
---

<intent-contract>

## Intent

从已累计首页续播、播放方式、音量和倍速修复的独立发行源码构建一次 Windows amd64 和 macOS Universal 本地 smoke 批次。用当前前端生产构建运行回归并打包。逐项校验 Windows 安装器、便携程序、macOS 归档和随包文档的 SHA-256，在可用架构启动最终包，把产物与验收记录交付到项目。批次使用本地工具链，不触发 GitHub 打包。

</intent-contract>

## Implementation Notes

本轮使用上一批通过的本机打包命令，固定发行提交和前端哈希。工作只涉及产物验收与进度文档，使用单步路线。构建和回归并行时，浏览器写入了独立检出里的 QA 截图，打包脚本末尾的干净树检查提前退出。归档、macOS 临时签名和双平台哈希核对已先完成。回归结束后还原测试截图，隔离 Wails 临时文件，重跑干净树与前端哈希门禁。确认测试产生的改动均在 design/rendered，二进制嵌入的 68 个 frontend/dist 文件未变化。

## Plan Change Log

## Review Triage Log

### 2026-11-26 — Review pass

- verdicts: 1 finding — high 0, medium 0, low 1, false 0, maybe-false 0
- findings:
  - `[low]` `[patch]` 快审时产物仍在隔离目录，候选 STATUS 指向交付路径。发布到项目的本地测试目录后重新逐项核对 SHA-256，再更新项目 STATUS 和 BMad 记录。

## Verification

- 前端生产构建、类型、全量 Vitest 和 Chromium，后端 Go vet、Go 全量，记录打包前后前端文件哈希。
- 两平台独立 SHA-256 校验，逐项检查安装器成员、两个 macOS 切片、最低版本和本机原生归档启动。
- 三份二进制 Go 漏洞扫描及 npm 生产依赖审计。记录验证范围和仍待执行的平台与签名验收。
- 打包源码树、68 项哈希一致。Go 二进制漏洞扫描、npm 审计及 365 项浏览器回归通过。独立工具校验 13 项，通过故障注入拒绝篡改副本。

## Auto Run Result

- 当前发行源码构建 .40 的 Windows amd64 安装器、便携版、macOS Universal ZIP 及配套清单。项目 STATUS 与 BMad 记录指向对应本地测试产物。
- 快审处理 1 个 low，复制产物到项目并对最终路径再校验。无延期或追加评审事项。
- 68 个前端文件在打包前后与回归用生产文件一致。122 项 Vitest、365 项 Chromium、Go vet/全量、12 项发行工具、11 项 macOS 工具通过。两平台 13 项 SHA-256、篡改拒绝、三份二进制漏洞扫描及 npm 生产依赖审计通过。
- Apple Silicon 从最终 ZIP 正常启动和退出，隔离 SQLite 完整。当前主机无法执行 x86_64；Windows 与 Intel Mac 原生、签名和公证、真实授权片源联调仍待验收。没有运行 GitHub 打包。
