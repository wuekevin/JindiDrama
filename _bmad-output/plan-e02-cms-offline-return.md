---
title: '苹果 CMS 故障时直接返回已加载页'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: fef4540b30200e28d0f09ce50bfed1191345c746
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

苹果 CMS 翻页失败后，返回已加载页应恢复页面的结果和分页操作。片源仍不可用时，返回应直接使用同一浏览范围内的内存结果。用户继续翻页或重试时照常请求片源。

</frozen-after-approval>

## Implementation Notes

只在点击「返回第 N 页」且路由匹配时使用现有结果。恢复保存的分页标记。浏览器刷新仍执行正常读取。已加载结果跟随页面、片源和浏览范围管理。

## Verification

- 用整个目录接口持续失败的夹具复现，再核对返回前后不同页、焦点、URL 和调用次数。
- 跑搜索与片源目录浏览器回归，以及前端类型检查、生产构建和单元测试。


## 实际验收

- 旧构建在持续断线时返回第 1 页，未恢复「下一页」，回归用例失败。修复版验证第 1、2 页均可恢复，返回时无新增目录请求。重新翻页和恢复连接后的重试按 URL 访问正确页码。
- 38 项搜索与片源 Playwright、122 项 Vitest、类型检查、生产构建、Go vet 及 Go 全量测试通过。独立重跑关键用例并对照搜索设计稿核对恢复结果。
