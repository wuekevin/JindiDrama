---
title: 'E13 正式打包必须通过同一提交的功能回归'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'dd9c000790bf6195a38d7f464dd82b49dcf64db1'
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

正式交付应证明同一提交在 Windows 和 macOS 同时通过前端与 Go 全量功能回归。正式打包在签署成品前运行这两类回归。交付核验要求两个原生打包任务均有成功的对应记录，且早于构建签名成品。原有测试包流程继续独立运行。

</frozen-after-approval>

## Implementation Notes

复用 CI 的类型检查、单测、Chromium 浏览器测试及 Go vet、全量测试命令。浏览器检测复用本次打包的锁文件构建。本机重启后从原始发行提交重建隔离工作树。浏览器用例会重写已跟踪的设计截图。托管发行回归在一次性检出内恢复该路径，并检查是否还有源码改动，确保编译和签名从干净源码开始。

## Verification

- 发行 Node 测试覆盖两平台成功、回归记录缺失或失败、步骤顺序错误。
- actionlint 检查正式与测试工作流。用本机工具链运行与正式门槛一致的命令。
- 使用新脚本复验留存的 .52 Windows 和 macOS GitHub 原始测试包。

## Results

- Node 22 发行测试 18 项通过。新增断言分别删除、设为失败或跳过两个平台的前端及 Go 回归记录，并验证错误的步骤顺序会被拒绝。旧版在该回归中失败。
- Node 22 锁文件安装、生产构建、类型检查、125 项 Vitest 和 404 项 Chromium 全量通过。Go 工具链通过依赖校验、vet 和全量回归。已恢复隔离工作树内的 47 份测试生成的跟踪截图。
- 两个工作流通过 actionlint。运行工作流提取出的 Bash 完整步骤，验证正常截图生成、跟踪源码改变、未跟踪文件新增及浏览器失败。新脚本仍可核对 .52 两份 GitHub 原始 ZIP 和两平台的内部清单。
- GitHub 本轮只读。正式签名、公证和原生跨平台成品待在托管环境验收。


## Review Triage Log

- medium，已修复。Go 在包列表模式下会复用成功的测试结果。正式门槛使用 `go test -count=1 ./...`，本机也用相同参数完成全量。
- medium，已修复。Go 嵌入的 `frontend/dist` 受忽略规则排除。正式门槛对包括隐藏文件在内的路径与字节计算 SHA-256，在浏览器回归后和 Go 回归后验证。夹具拒绝两阶段各自造成的改写。实际浏览器使用的产物与独立编译的产物摘要一致。
- low，已修复。在两平台分别将前端和 Go 回归移到原生构建之后、签名之前，核验必须拒绝该顺序。
