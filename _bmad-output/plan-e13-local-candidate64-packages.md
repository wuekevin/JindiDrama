---
title: 'E13 .64 累计源码的成品与跨版本资料验收'
type: chore
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: 'bc397d67fbd7b30a25a62c1f9137218cd2d97ce0'
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

用当前累计发行源码打包 Windows x64 与 macOS Universal 测试候选。两平台从最终目录复验 SHA-256，Apple Silicon 从实际 ZIP 验证启动、回滚和读取旧候选资料。保留准确的 Windows 和 Intel Mac 原生、正式签名、公证和授权上游验收边界。

</frozen-after-approval>

## Implementation Notes

复用隔离发行源码、锁文件构建与 NSIS/Wails 工具。使用旧测试资料副本验收新归档。保留两平台原始交付文件、来源和各检查日志。

## Verification

- 重建锁文件依赖，核对最终归档及 Windows 安装器载荷。Mac 双切片最低系统为 13.0。
- 完整 Chromium、Vitest、Go 全量、竞态与对应 WebKit 专项。核对生产依赖和成品漏洞扫描。
- Apple Silicon 解包启动、退出及回滚。用真实 .64 应用读取旧候选的资料副本，验证设置与回读。

## Results

- Windows x64 安装器、便携版和 macOS Universal ZIP 已从发行源码 `bc397d67fbd7b30a25a62c1f9137218cd2d97ce0` 构建，两平台合计 13 项 SHA-256 复验通过。三种架构的内嵌前端逐项核对为 69 个文件，Windows 安装器解出的主程序与便携版一致。
- 真实 Apple Silicon 成品启动、正常退出及 WAL 回滚通过。使用 .63 的实际资料副本验收预发布渠道设置保留。真实设置表单拒绝无效 HTTP 地址。原资料数据库和当前 ZIP 摘要未变化，副本 SQLite 完整性通过。
- Chromium 409 项、Vitest 125 项、WebKit 专项 8 项与 Go 全量、vet 和相关竞态检查通过。完整 WebKit 回归为 398 项通过、10 项失败、1 项重试后通过。9 个用例调整脚本后通过专项复跑。片库分集重试的键盘路径通过，鼠标焦点待核实。片库搜索进入播放的 ResizeObserver 警告在独立复跑再次出现。测试脚本调整保留在隔离环境供后续修复。
- Windows 与 Intel Mac 原生运行、当前源码 GitHub 托管包、正式签名公证和授权公网联调仍待验收。

## Review Triage Log

- medium，修正验收说明。原鼠标点击重试连续两次未恢复分集区域焦点。专项复跑显式聚焦重试按钮并按 Enter，只证明键盘路径通过。鼠标路径保留为待验收。
- medium，修正验收说明。全量 Playwright 仅重试了授权片库搜索进入播放用例。独立复跑又出现 ResizeObserver 警告，记录保留该证据及根因待查状态。
