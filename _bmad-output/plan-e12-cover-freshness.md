---
title: 'E12 复用明确允许缓存的 CMS 封面'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '32ecd56f9d0433794fad54c0ef9249ec4cc73dcd'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

重复浏览 CMS 剧目时，本机封面中转会重复请求并解码同一张图。对明确公开且仍在上游新鲜期内的封面，重复访问同一张有效票据时复用校验后的 JPEG，减少上游请求和解码。

带查询参数、带响应 Cookie、需要按请求变化或缺少明确缓存许可的封面仍走原有请求。进程内缓存有总量上限，票据失效或片源关闭后失效。

</frozen-after-approval>

## Implementation Notes

复用封面票据生命周期和现有数据验证。WebView 仍收到 no-store，封面副本只在 Go 进程内保留。[HTTP 新鲜度](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.2.3)

进程内保留已校验的 JPEG，容量上限 8 MiB。依据上游 Date、Age 和 max-age 计算最早到期时间。超过票据期限后原票据与副本一起移除。

## Plan Change Log

## Review Triage Log

- 刷新后复用旧图。显式刷新会以新响应更新或清除原副本，请求 no-store 时只清除。
- 较早启动的慢请求可能覆盖新图。只接受同一票据最新请求的写回。
- 容量可能被陈旧副本占住。新图写入前遇到容量不足时回收过期数据。

## Verification

- Go 封面用例验证请求计数、缓存命中、刷新、上游许可、过期与容量。
- Go 全量测试、vet、App 竞态和 Windows amd64 交叉编译。
- 上述检查均通过。封面请求夹具确认普通重复请求只访问上游一次，显式刷新替换旧图。并发写回顺序、容量回收和关闭清空通过专项检查。
