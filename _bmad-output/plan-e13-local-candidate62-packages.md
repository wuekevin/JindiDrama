---
title: 'E13 .62 发行候选的本机双平台验收'
type: chore
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '9ad65eb70abc3878d5c294848997cc848f990f35'
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

用 .61 之后的累计发行源码生成可在本机交付的 .62 Windows x64 与 macOS Universal 测试包。两个平台打包和 Chromium 全量回归均使用一份锁文件构建产物，并通过内嵌文件校验。复算两平台完整 SHA-256，验收 Windows 安装器内容和 Apple Silicon 真实启动、退出与 SQLite 回滚。记录哪些门槛必须在 GitHub 或真实授权环境完成。

</frozen-after-approval>

## Implementation Notes

使用独立发行提交、Node 22 和仓库锁定的 Wails 版本。Windows 交叉构建。macOS 使用独立的本机测试包标识和临时签名，以隔离 WebView 配置。首次 NSIS 版本检查使用了瓶子编译时的默认目录，显式设置隔离 NSIS 资源目录后从头重跑。

## Verification

- Windows 安装器内容与独立构建输入一致，macOS 两切片和最低系统版本正确，确认所有二进制含同一前端。
- Apple Silicon 从最终 ZIP 启动、退出并核对 SQLite 和旧版回滚。
- Chromium 全量、Vitest、Go 全量及发行脚本回归。复核浏览器运行前后的前端摘要。
- 最终归档位置核对 13 项 SHA-256 及随包说明。

## Results

- 从发行提交 `9ad65eb70abc3878d5c294848997cc848f990f35` 构建两平台成品。Windows 安装器包含 7 个预期文件，主程序、WebView2 和 4 项 NSIS 资源与输入逐字节一致。macOS 双架构最低系统版本均为 13.0。
- 核对三个架构的完整 Go 内嵌文件表，包含 69 个文件和 5 个目录。按路径读取实际数据并比对文件字节和 Go 编译器摘要，旧 .61 的主程序被拒绝。Mac 的两个 Mach-O 切片均使用偏移形式链式重定位。Wails 对 `wails.json` 仅重排格式，解析配置未变，已在隔离检出恢复原文件。
- 浏览器使用同一份锁文件构建产物，构建、打包和浏览器回归后摘要一致。Chromium 两轮各 406 项、WebKit 5 项、Vitest 125 项、Go 全量和 vet、Node 发行测试 18 项通过。三架构二进制和 npm 生产依赖扫描未报告已知漏洞。
- Apple Silicon 从本机测试 ZIP 启动并正常退出，隔离数据库和活动 WAL 回滚通过。两平台文件复制到固定候选目录后，独立复算 13 项 SHA-256。随包说明已包含 CMS 设置入口和回滚说明。正式签名、公证、Windows 与 Intel Mac 原生运行、签名在线升级及授权公网联调待验证。


## Review Triage Log

- medium，已补齐。浏览器第二轮 406 项通过，记录了环境、工作目录和严格端口上的 Vite preview 启动命令。使用同一个运行中的服务逐文件读回 69 个 HTTP 响应，与内嵌文件逐项核对。
- medium，已补齐。Apple Silicon 再次直接从最终归档目录运行，原生启动、退出和回滚前后的 ZIP 摘要相同，并匹配归档清单。
- low，已补齐。状态记录明确产物和证据存放在 JindiDrama 主工作区的 `build/bin/`。
