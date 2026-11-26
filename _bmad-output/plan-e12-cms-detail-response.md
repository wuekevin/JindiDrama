---
title: 'E12 苹果 CMS 详情与分集共用响应'
type: bugfix
ticket: ''
created: '2026-11-26'
status: built
baseline_revision: 'ee558513621c4211c884bd529470356caa14f63b'
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

打开苹果 CMS 详情时，桌面分别请求剧目与分集，片源更新期间可能将两个不同版本拼在一起。新桌面桥用一次请求读取并校验同一份详情与分集。保留旧桥兼容和独立的最新播放票据校验；封面仅通过本机地址交给页面，错误时允许用户重试。

</intent-contract>

## Implementation Notes

沿用 Provider 的详情解析和 App 封面处理，前端可选探测新桥，使用单步路线。沿用已有 Go 详情解析、封面代理和播放校验。浏览器红测在保留新桥但禁用旧接口时可复现详情重试失败。

## Plan Change Log

## Review Triage Log

### 2026-11-26 — Review pass

- verdicts: 1 finding — high 0, medium 0, low 1, false 0, maybe-false 0
- findings:
  - `[low]` `[patch]` 固定分集状态只能验证取票多一次 GET。已在桥接确认第 3 集可播后从 HTTP 夹具移除该集，断言取票返回不可播放，且无地址或票据。

## Verification

- Go 夹具确认单次上游 GET、剧目与分集一致、封面地址隔离、陌生片源 ID 拒绝及后续取票仍需最新数据。
- 浏览器验证新桥用于详情和播放，失败可重试，旧桥专项全通过。运行前端类型、生产构建、Vitest、Go vet 与全量测试。
- Apple Silicon 使用 Wails 与隔离 HTTP 夹具，逐步记录页面操作产生的上游请求数量、正常退出和 SQLite 状态。
- 实测 122 项 Vitest、59 项 Playwright、Go vet/全量、Windows 交叉构建通过。Apple Silicon 原生详情一次 GET，播放页为新详情和取票各一次 GET。

## Auto Run Result

- `applecms.go` 从同一条响应生成剧目与分集，旧分集绑定也复用投影。`apple_cms_config.go` 将合并读取暴露到 Wails，仍通过封面代理。前端在苹果 CMS 路径优先使用合并方法并兼容旧桥。
- 快审补充 1 个 low，验证已经在详情显示可播的剧集下架后，重新取票拒绝并无媒体地址。无延期项。
- 类型检查、生产构建、122 项 Vitest、59 项目录与播放 Playwright、Go vet/全量、Windows amd64 交叉构建通过。Apple Silicon 原生详情只发 1 次请求，播放页读取详情并单独取票，第 3 集实际解码，正常退出与独立 SQLite 验证通过。
- 当前验收使用合成授权片源。真实公网授权端点、Windows 与 Intel Mac 原生、签名及公证仍待完成。未运行 GitHub 打包。
