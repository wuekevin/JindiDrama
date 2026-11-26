---
title: '苹果 CMS 详情页同类短剧'
type: feature
created: '2026-11-27'
status: built
baseline_revision: da7442929dc4816496203712a5649cfdb09b9089
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

让当前苹果 CMS 剧目的详情页可以继续浏览同片源且同分类的短剧。分类缺失或请求失败时详情和选集仍可用。

</frozen-after-approval>

## Implementation Notes

从已读取的剧目记录取得分类 ID。复用现有 CMS 分类接口、封面映射和推荐卡片布局。限定为分类首页中最多六个同源可播条目。响应与发起请求时的详情版本绑定。

## Verification

- Go Provider 单测及全量测试。
- Vue 类型检查、生产构建和 Chromium 详情回归，覆盖异源、重复、不可播、切换详情后的迟到响应。
- 用详情设计稿对照默认尺寸，并检查 220% 窗口布局。

## 实际验收

- 剧目分类 ID `007` 规范为 `7`。浏览器验证分类 7 第一页请求、同源过滤、无可播放条目过滤、去重、详情路由、过期回包和分类失败时选集仍可用。
- 对照默认详情设计稿核对卡片区域。1448×1086 和 1280×860、220% 的本机截图位于 `build/bin/qa-cms-related-dramas`。
- Apple Silicon 以合成接口验证 Go 到 WKWebView 的同源推荐和详情跳转。未触发媒体转发，退出后的数据库完整性通过且没有观看历史。
- 122 项 Vitest、81 项 Playwright、Go vet、Go 全量和专项竞态测试、Windows 生产交叉编译通过。
