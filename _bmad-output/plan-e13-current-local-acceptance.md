---
title: '当前发行源码本地集中验收'
type: chore
created: '2026-11-27'
status: built
baseline_revision: 7a6fd61ece30172dcf15f1d98c9751be2a0c765f
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

用当前隔离发行源码重建应用并执行全量本地回归。覆盖 .45 之后累计的 CMS 演职信息、翻页失败与恢复、钥匙串交互和发行核验，再以同一构建资源实际启动 Apple Silicon 应用。

</frozen-after-approval>

## Implementation Notes

使用独立源码目录和浏览器回归端口。macOS 测试应用使用独立标识和临时配置目录。验收日志归档后写入状态文档。

## Verification

- 前端类型检查、生产构建、Vitest、全部 Playwright 与生产依赖审计。
- Go 依赖校验、静态检查、全量测试，以及 App、CMS 和钥匙串相关竞态测试。
- 使用 Apple Silicon 实际运行本地构建，验证应用版本、退出、SQLite 与回滚。
- 运行发行工具测试和源码历史验证。


## 实际验收

- 完整浏览器回归 387 项与 Vitest 122 项通过。评论和弹幕的截图此前可能在播放器加载时生成。两项截图测试等待暂停就绪后拍摄，变更后两个套件 54 项通过。十页重新与设计稿对照生成差异图。
- Go 静态检查、全量、App/CMS/钥匙串竞态测试通过。第一次并行检查遇到磁盘空间不足。校验并清理 .45 临时解包副本后串行重跑，Go 源码漏洞检查使用独立缓存完成。
- Apple Silicon .46 测试 ZIP 验证启动、正常退出、SQLite 完整性和回滚。Windows x64 仅执行生产交叉编译，以及用夹具验证会话和签名脚本。源码、两个二进制和 npm 生产依赖未报告已知漏洞。
- 两个平台的签名、公证、正式草稿、Windows 及 Intel Mac 当前源码原生运行和授权片源仍待验收。
