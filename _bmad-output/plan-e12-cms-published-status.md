---
title: 'E12 按苹果 CMS 上架状态过滤目录和播放'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e2068c2385f64599d5da4d7155d0b74256dca604'
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

授权苹果 CMS 的响应明确提供 vod_status 时，仅将状态 1 的剧目视为已上架。目录、关键词、分类和直接详情不能展示已下架剧目。播放时根据最新详情重新判断上架状态，拒绝新的整集票据。保留省略此字段的兼容片源。过滤后内容为空的分页仍使用已核对的上游页码和总页数继续浏览。

</frozen-after-approval>

## Implementation Notes

沿用 Provider 对分页、分类和最新详情的验证入口。用合成片源验证目录筛选和从上架切换到下架。

- 旧版复现下架内容出现在目录及签发票据。修复后的 Go 适配器依据上架字段过滤并重新核对详情。
- 首轮浏览器测试发现桥接层丢弃空页的有效下一页标记。修复后通过 81 项目录及播放检查。122 项 Vitest、Go 全量、vet 及 App/CMS 竞态通过。

## Plan Change Log

## Review Triage Log

## Verification

- `go test -p 1 ./internal/provider/applecms ./internal/app` 验证目录、搜索、分类、详情及播放入口。
- `go vet -p 1 ./...`、`go test -p 1 ./...`、`go test -p 1 -race ./internal/provider/applecms ./internal/app` 验证回归。
- Chromium 目录专项确认空页仍可按已验证的分页信息进入下一页。
