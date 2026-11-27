---
title: 'E13 诊断日志遮蔽带引号的凭据'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'a46a667334669c2dbf7568147e8a6e7a788d0d6b'
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

诊断日志应遮蔽带引号的键值凭据，包括历史日志导出。双引号和单引号中的空格、分隔符、转义字符和 URL 都属于凭据值。无效或截断的带引号值在本行内从严遮蔽，正常闭合的字段保留后续诊断内容。

</frozen-after-approval>

## Implementation Notes

沿用新日志写入和历史诊断导出的现有 Redact 入口。从旧版本可复现的诊断格式建立回归测试。

- 旧版回归复现新日志含带引号的凭据。修复后覆盖引号、空格、转义、URL、内含请求头字样和截断值。已核对当前日志实际写盘及上一代文件导出。
- Go vet、全量、App 与诊断模块竞态均通过。Windows x64 诊断包测试程序交叉编译成功。

## Plan Change Log

## Review Triage Log

## Verification

- `go test -p 1 ./internal/observability` 验证引号、转义、截断和写入及旧日志导出。
- `go vet -p 1 ./...`、`go test -p 1 ./...`、`go test -p 1 -race ./internal/observability ./internal/app` 验证回归。
