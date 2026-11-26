---
title: 'E12 苹果 CMS 目录限流冷却'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '30e62a5b3395ee3508f8a0cf75f0fedecee696eb'
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

上游目录返回 HTTP 429 时，苹果 CMS 后续目录、搜索、分类、详情和取票核验请求应尊重 Retry-After。当前片源实例共享冷却时间，冷却期间立即返回固定错误，冷却结束后允许请求。支持秒数与 HTTP 日期，对缺失或异常值采用短暂缺省冷却。读取已签发播放票据的媒体转发独立运行。并发响应不能把已知冷却提前。

</intent-contract>

## Implementation Notes

目录业务统一通过 Provider 的 HTTP 请求入口。新增实例内冷却，不自动重试。单步路线覆盖生产请求和真实 HTTP 夹具。分类入口在进入刷新锁前检查；实际 HTTP 发送仍统一经过请求层。超大秒数避免转成负的 Go 时间长度。

## Plan Change Log

## Review Triage Log

### 2026-11-27 — Review pass

- verdicts: 3 findings — high 0, medium 1, low 2, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` Wails 取票层和播放页会隐藏 CMS 限流提示。保留唯一已知错误，播放页只允许该固定文字。Go 取票、Chromium 和原生 Wails 分别验证。
  - `[low]` `[patch]` 十进制秒数刚超出无符号范围时落入缺省 5 秒。纯数字溢出采用最长可表示的整秒冷却，拒绝包含其他字符的超大值。
  - `[low]` `[patch]` 序列化截止时间更新不等同于同时发出的 429。把记录改为准确的截止时间比较，并补上线真实 HTTP 的 1 秒自然到期测试。

## Verification

- 本机 HTTP 夹具验证 429 后跨入口不发请求，冷却结束后可恢复，并核对片源实例隔离。
- 时间规则覆盖秒数、HTTP 日期、过期时间、零和异常值。运行 Go vet、全量 Go、Provider/App 竞态和 Windows amd64 生产交叉构建。
- 旧实现遇 429 只返回目录错误。新夹具覆盖真实请求数、结束冷却后恢复以及另一个 Provider 不继承。时间与 HTTP 夹具重复五次通过。

## Auto Run Result

- HTTP 429 在当前片源实例共享冷却。取票限流提示贯通 Wails 和 Vue，冷却到期后允许用户重试。
- 快审修复 1 个中风险和 2 个低风险发现，无推迟项。
- 生产前端构建、类型检查、122 项 Vitest、61 项目录与播放 Playwright、Go vet、Go 全量、Provider 和 App 竞态检查及 Windows amd64 生产交叉构建通过。
- Apple Silicon Wails 夹具收到 HTTP 429 后显示提示。等待 Retry-After 后重试，实际解码第 3 集。退出后 SQLite 完整性检查通过。真实授权公网片源、Windows 和 Intel Mac 原生运行及正式签名仍待验收。
