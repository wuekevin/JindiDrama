---
title: '苹果 CMS 翻页失败时保留已加载结果'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 56787e139bff47c7f25069ea5a6f817f804040cf
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

苹果 CMS 翻页失败时，同一浏览范围的上一份有效结果应继续可见。明确当前展示页和失败页，允许重试或返回，保留 URL、回退和取消过时响应的行为。

</frozen-after-approval>

## Implementation Notes

在搜索页实例中保留一个成功的 CMS 页面。目录、分类和关键词各使用各自的范围。切换范围、片源或确认分类失效时清空快照。直接打开未加载过的失败页继续显示错误态。

## Verification

- 浏览器覆盖目录、关键词和分类的失败、重试、旧卡片及页码、切换范围和历史导航。
- 复核默认与 220% 搜索布局。


## 实际验收

- 前端类型检查、生产构建及 122 项 Vitest 通过。Go vet 与全量 Go 测试通过。
- 第一次浏览器回归发现一条已有断言仍检查全页错误态。更新断言核对保留的页面，再次运行搜索和片源目录套件，38 项全部通过。
- 对照搜索设计稿，检查默认布局及 1280×860、220% 缩放下的提示换行。浏览器使用合成片源。
