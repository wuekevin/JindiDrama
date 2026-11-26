---
title: '苹果 CMS 详情展示主演和导演'
type: feature
created: '2026-11-27'
status: built
baseline_revision: 7f89f06b5e0bf4ca1304f49d57721851568f4927
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

让苹果 CMS 详情展示现有请求里实际返回的主演和导演。缺失、无效或含链接的上游数据不妨碍剧目及选集显示。

</frozen-after-approval>

## Implementation Notes

同一条已核验的详情记录返回可选主演和导演。Go 清理标签及地址、分隔姓名、去重并限制显示数量。Vue 按纯文本展示。演示主演仍使用原样式和搜索入口。

## Verification

- Go 夹具验证演员和导演字段、清理、边界以及非字符串数据不阻断详情。
- Chromium 验证详情、默认和 220% 布局，以及切换至无演职信息作品后隐藏相应行。

## 实际验收

- Go 夹具验证字段来自同一条详情、各字段最多 12 个名字、乱码结构忽略、混合分隔符清理、HTML 和链接去除。
- Chromium 复核默认和 220% 窗口下长姓名换行、切换剧目后隐藏旧资料。82 项详情与播放回归和 122 项 Vitest 通过。
- Apple Silicon 经本机合成接口验证原生 Wails 取数、WKWebView 显示、同源跳转和 SQLite 完整性。Go 全量、专项竞态和 Windows x64 生产交叉编译通过。
