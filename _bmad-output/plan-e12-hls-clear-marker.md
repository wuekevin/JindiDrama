---
title: 'E12 支持 HLS 明文分段声明'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'c44260e8a378c7b37c7c427d4c56d599d5793aa5'
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

让授权点播的 HLS 媒体清单兼容 RFC 8216 的明文标记，保留转发安全限制。只接受媒体清单中精确的 METHOD=NONE 声明。包含任何其他加密方式、密钥属性或无效声明时整份清单失效。

</frozen-after-approval>

## Implementation Notes

沿用现有验证并原子发布清单的路径。复用自制 fMP4 样片验证当前源码的原生 WKWebView 播放。

- 修复前 Go 中转测试收到 502。修复后保留明文标记，带加密声明或其他属性的清单保持拒绝。Go vet、全量、专项和竞态测试通过。
- Apple Silicon WKWebView 从目录打开自制 fMP4 分集，确认播放推进、正常退出和隔离 SQLite 完整性。原生应用使用仅用于验证的测试传输。

## Plan Change Log

## Review Triage Log

## Verification

- `go test -p 1 ./internal/app -run TestCatalogHLS` 验证合法明文声明与拒绝异常清单。
- `go vet -p 1 ./...`、`go test -p 1 ./...`、`go test -p 1 -race ./internal/app -run TestCatalogHLS` 验证回归。
- 通过原生 Apple Silicon 桌面执行目录、详情、播放并验证实际播放推进及 SQLite 完整性。
