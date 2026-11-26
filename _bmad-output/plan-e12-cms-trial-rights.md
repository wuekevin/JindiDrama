---
title: 'E12 按苹果 CMS 试看字段限制整集播放'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '83e01c4480e2143431455fb3ec30e42142cafcf1'
route: oneshot
route_source: pinned
risk: medium
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

苹果 CMS 可使用 vod_trysee 标记试看时长。适配器应保留短剧目录和详情，对有试看限制或该字段格式无效的条目阻止整集播放。申请播放时以最新详情为准。省略该字段或使用空值、零值时继续按原有授权条件处理。

</frozen-after-approval>

## Implementation Notes

沿用 Provider 的现有播放限制入口。验证从目录加载到播放申请期间发生权限变更时，桌面应用不签发媒体转发票据。

- 旧版本在 Provider 与桌面绑定中均可复现权限遗漏。修复后，分集状态、票据和媒体请求均遵循最新详情。Go 专项测试、Go vet、全量测试和 App/CMS 竞态测试通过。

## Plan Change Log

## Review Triage Log

## Verification

- `go test -p 1 ./internal/provider/applecms ./internal/app` 验证试看字段与现有授权模式。
- `go vet -p 1 ./...`、`go test -p 1 ./...` 和 `go test -p 1 -race ./internal/provider/applecms ./internal/app` 验证应用与 Provider 回归。
