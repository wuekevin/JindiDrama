---
title: 'E12 合并苹果 CMS 分类并发刷新'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '2c07cfd08fbe3ca5f985c377b60e33940e5801d3'
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

同一个苹果 CMS 片源被连续浏览时，并行的分类刷新和分类校验共享一次正在运行的分类请求。等待已取消时立即结束。若原请求因自己的上下文取消，仍有效的请求可重新刷新。保留已有的分类白名单、刷新后移除旧分类、限流和错误处理，成功刷新后更新同一份目录快照。

</frozen-after-approval>

## Implementation Notes

分类网络请求目前在 genreMu 内串行执行。改用一次进行中的刷新状态，短暂持锁访问快照和状态，等待独立地响应 context。保持成功快照与错误结果的边界。

## Verification

- 用本机受控 HTTP 响应复现已取消的分类请求仍被占用。
- 验证多次分类查询合并、有缓存的分类校验等待最新分类、取消后可重新刷新并正常显示、错误和限流照旧。
- Go 全量及竞态测试、Chromium 分类页面回归。

## Results

- 修复前，分类刷新占用互斥锁期间，已取消的查询和分类分页请求不能退出。修复后，已取消的请求立即返回，已加入的等待者也可独立离开。
- 单轮上游只发一次分类请求。成功刷新淘汰已移除分类，失败保留已成功的快照，收到 429 时所有并发等待者停止后续网络请求。发起方取消后仍有效的请求可完成刷新。分类列表以独立副本返回。
- Go 分类专项连续 25 轮，Go 全量与 vet，Provider 和 App 竞态及 Windows amd64 测试交叉编译通过。Chromium 目录模块 18 项通过。Windows 运行和授权公网 CMS 仍待验收。

## Review Triage Log

- 中，已修复。已加入的分页校验在发起方取消后重新读取分类，测试证明已删除分类不会发出请求。
- 中，已修复。保留同轮已经收到的 429，包含 Retry-After 为 0 且发起方取消的场景。等待期间若其他 CMS 请求触发限流，也继续返回限流状态。
- 验收范围以 Go 受控 HTTP 案例证明真实并发行为。浏览器通过桌面桥夹具覆盖页面选择、重试及过时结果处理。
