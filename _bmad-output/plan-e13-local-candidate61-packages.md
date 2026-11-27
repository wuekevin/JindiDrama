---
title: 'E13 .61 累计本地打包验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'ee62daf4e95d3d385d310550655c2ec089a8d6a4'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: [_bmad-output/plan-e13-local-candidate60-packages.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

从同一独立发行源码生成累计的 .61 Windows x64 和 macOS Universal 测试候选。纳入 .60 后的首页续看恢复、更新下载校验和清理、公证诊断。验证存档 SHA-256、成品内容及 Apple Silicon 原生运行。

</frozen-after-approval>

## Implementation Notes

沿用 Wails、NSIS 及归档脚本。从发行提交重建前端。Node 依赖锁与 .60 相同，复用本机按锁文件安装过的依赖。独立目录验收。

## Verification

- 两平台 13 项清单，实际 Windows 便携版、安装器内部文件及 macOS 两个架构比对预期内容。
- Chromium 全量及 WebKit 首页、设置与 Provider 重点场景，核对生产前端、Go 和发行脚本。
- Apple Silicon 从最终归档启动、退出并验证 WAL 回滚及 SQLite。正式模式确认临时签名候选被拒绝。

## Results

- .61 双平台测试包从所记录的同一发行源提交构建，核验最终归档 Windows 7 项、macOS 6 项 SHA-256，并实际执行随件 PowerShell 与 sh 校验命令。Windows 安装器的主程序及 WebView2 引导程序逐字节核对。macOS 两切片最低为 13.0。
- Apple Silicon 从最终 ZIP 启动真实窗口，正常退出并通过 SQLite 和活动 WAL 回滚验证。macOS 临时签名归档在正式验证模式下被公证票据门禁拒绝。Windows 安装器和便携版均无 Authenticode 签名。
- 生产前端 69 条路径在构建前后、三架构嵌入后及 Chromium 使用的临时卷中逐项一致。Chromium 全量最终 404 项通过，WebKit 专项 13 项、前端单测 125 项、Go 全量/vet/竞态、Node 发行脚本 17 项、macOS 工具 18 项和 Windows PowerShell 夹具通过。
- Chromium 首轮写截图遇到 ENOSPC，专项连跑四次通过。第二次全量同样被空间不足中断，之后在单独内存卷上全量通过并卸载该卷。WebKit 原用例使用 Chromium 的 Tab 行为，改用现有 macOS WebKit 辅助键，测试改动晚于打包提交且未改变生产前端。
- PowerShell 恢复时遇到磁盘空间不足，验证 Microsoft 公布的归档摘要后恢复同一版本。三架构二进制和 npm 生产依赖扫描未报告已知漏洞。签名环境、仓库级 secrets 和 Releases 的只读计数均为 0。

## Review Triage Log

- 已补验。Windows NSIS 的预期完整清单为七项，逐字节核对主程序、WebView2 和四项工具链资源。提取出的卸载器通过 PE 类型及 NSIS 自身完整性检查。Windows 上的卸载运行仍待原生验收。
- 已修正记录。最终归档和日志存于本机 JindiDrama 主工作区的 `build/bin/`，独立发行检出引用同一组证据。
