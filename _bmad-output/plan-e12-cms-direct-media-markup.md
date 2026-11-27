---
title: 'E12 拒绝伪装成直链视频的上游错误页'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '54700f49580b7ef728e85a13bbcdcc191e8abde4'
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

授权片源的直链 MP4 和 WebM 不应转发上游伪装为视频的 HTML 或 XML 错误页面。复用有界正文前缀检查，在发送媒体响应前识别错误页。请求其他备用源时应能继续读取正常媒体。

</frozen-after-approval>

## Implementation Notes

复用现有 HLS 检查和媒体转发机制。直链 Range 的二进制内容可以从中间字节开始，按现有的响应区间、完整性和超时规则转发。

- 修复前得到 200 / 206 与错误正文。修复后三种响应返回空正文 502，备用源可用。隔离测试比对两种自制样片的全文、探测 Range 和中间 Range。Go 媒体专项、全量、vet 与 App 竞态通过。

## Plan Change Log

## Review Triage Log

## Verification

- 合成上游返回正确媒体头与错误页正文。直链回归核对 502、正文为空、备用源及 Range 读取。
- `go vet -p 1 ./...`、`go test -p 1 ./...`、`go test -p 1 -race ./internal/app` 验证媒体转发、超时、回退及 HLS 既有回归。
