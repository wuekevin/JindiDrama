---
title: 'E13 更新下载失败提示与重试'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'a41c05e39a278386d01268375fffcfc196fcb686'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

安装包下载失败时，设置页应根据出错原因给出对应操作。服务限流、临时异常、安装包下线和本地写入失败需要各自的提示，下载取消或失败后应能再次尝试。

</frozen-after-approval>

## Implementation Notes

沿用现有更新器的 SHA-256 校验、暂存文件清理和设置页重试流程。修改和验收范围集中在现有下载入口与界面错误反馈。

- 区分 GitHub 响应、连接与传输超时，以及本地暂存写入失效。设置页沿用现有错误提示与重试按钮。
- Go vet、Go 全量测试、下载器与应用竞态测试、前端生产构建和 44 项设置页 Playwright 通过。浏览器通过桌面桥夹具验收。

## Plan Change Log

## Review Triage Log

## Verification

- `go test -p 1 ./internal/update ./internal/app` 验证下载状态、本地目标写入异常、取消清理和文件完整性。
- `go vet -p 1 ./...`、`go test -p 1 ./...` 和 `go test -p 1 -race ./internal/update ./internal/app` 验证应用与下载服务回归。
- `npm run test:e2e -- tests/e2e/10-settings.spec.ts` 验证取消、写入错误、按钮恢复和下一次下载保存成功。
