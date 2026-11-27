---
title: 'E13 .59 累计本地打包验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'fefccb64d2c497752a79c0f08c22faf883ca3353'
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

从包含 .58 之后所有本地修复的发行提交构建一批 Windows x64 和 macOS Universal 测试包。验收发布候选中实际嵌入的前端、归档程序、回滚及两平台完整的 SHA-256 清单，为集中原生验收准备可追溯的候选。

</frozen-after-approval>

## Implementation Notes

使用现有 Wails、NSIS、随包说明和清单脚本，构建在独立的源码检出中进行。成品以独立的本地测试版本 .59 保存，记录三种架构的嵌入资源验证结果。

## Verification

- 在同一源码提交做前端生产构建、Vitest、Chromium 全量、WebKit 切源专项、Go 全量和 vet、发行脚本测试。
- 校验 Windows 安装器中的程序与便携程序一致，macOS 归档包含两种架构，三架构的 Go 嵌入文件逐路径比对生产前端。
- 再读回存档位置，对两平台清单做完整的项目校验与独立 SHA-256 验证。从最终归档验证 Apple Silicon 原生启动、正常退出、SQLite 完整性和活动 WAL 回滚。

## Results

- Windows x64 和 macOS Universal 从记录的同一 Git 提交构建。Windows 便携程序、NSIS 内主程序和 WebView2 引导程序通过比对，macOS 两种架构的最低系统版本均为 13.0。三种架构逐一核对嵌入文件集合与 69 项 SHA-256。最终位置的 Windows 7 项及 macOS 6 项清单验证通过。
- 前端构建、Vitest 125 项、Chromium 402 项和 WebKit 6 项通过。Go 测试及 vet、Node 发行脚本 17 项通过。Go 开发和生产构建、三套主程序以及 npm 生产依赖的扫描均未报告已知漏洞。
- Apple Silicon 从最终 ZIP 启动 Wails，正常退出，SQLite 完整性及活动 WAL 回滚通过。正式凭据、最新候选的 Intel Mac 和 Windows 原生验证、在线升级及授权公网联调待验收。
- 打包脚本最后的已跟踪文件检查碰到并行端到端测试生成的截图。确认只有隔离检出里的截图发生跟踪变更后，将其还原并重验已跟踪源码与发行提交一致，生产前端和成品清单均匹配。Wails 和 NSIS 的未跟踪生成物仍保存在隔离检出。

## Review Triage Log

- 低，已修正。`--untracked-files=no` 检查范围是已跟踪源码，报告已明确该范围及仍在隔离检出中的生成物。
