---
title: '.48 累计发行候选本地跨平台打包'
type: chore
created: '2026-11-27'
status: built
baseline_revision: 2262527a7da5c26acaa30aa3bf7f6719e9d288a7
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

从当前隔离发行源码使用同一份新构建的前端资源，生成 Windows x64 安装器、便携程序和 macOS Universal ZIP。执行全量浏览器回归、平台工具检查、产物 SHA-256 校验，并从最终 ZIP 在 Apple Silicon 启动与验证 SQLite 回滚。

</frozen-after-approval>

## Implementation Notes

前端截图回归使用独立副本，打包源目录保持干净。Windows 本地候选未签名。macOS 本地候选使用隔离标识和 ad-hoc 签名。正式签名及线上公证仍依赖发行凭据。

## Verification

- 运行全量 Playwright、Vitest、Go 检查及签名、组装工具测试。
- 检查 Windows NSIS 内文件、PE 元数据，macOS 两个切片及最低系统版本。
- 用项目校验器、独立 SHA-256 和随包指令复核。Apple Silicon 从最终 ZIP 启动、退出并验证 SQLite 回滚。


## 实际验收

- 391 项 Playwright、122 项 Vitest、Go vet、Go 全量和 App/CMS/更新/开机启动竞态通过。Windows PowerShell 夹具、16 项 macOS 工具和 13 项 Node 发行工具通过。
- 两平台 Windows 7 项和 macOS 6 项文件的清单均通过独立复核。NSIS 与 WebView2 内部文件匹配，macOS 两个切片的最低版本为 13.0。Apple Silicon 从最终 Universal ZIP 启动、退出，验证隔离 SQLite 及成品回滚。
- 检出和首次 Windows 编译因空间不足中断，清理隔离临时文件后恢复。macOS 解压验收使用同一份 ZIP 复验成功。Go 源码、三种二进制和 npm 生产依赖扫描未报告已知漏洞。
- 当前候选的 Windows 和 Intel Mac 原生、正式签名、公证及公网授权片源仍待验收。
