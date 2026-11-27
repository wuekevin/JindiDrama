---
title: 'E12 苹果 CMS 列表跳过异常记录'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '2cc862eda60dd45fb60da9dfe2c4a27652b0b699'
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

首页、搜索或分类的 CMS 地址返回正常条目和字段类型错误的条目时，正常条目仍可浏览。整页记录均无法解析时显示片源错误。详情和播放继续拒绝含异常记录的响应，保留页码、分类、上架和播放权限校验。

</frozen-after-approval>

## Implementation Notes

JSON 响应保持现有字节和记录数门槛。分条解析列表，仅对浏览列表跳过无法解析的成员。详情及取票共享完整响应校验，避免将含异常数据的响应当作已下架或从中签发票据。

## Verification

- Provider 覆盖混合记录、全部无法解析、有效空页、分类及详情异常。现有 App 夹具验证首页、搜索和后续 Go 环回播放。
- 运行 Go 全量、vet、CMS 与 App 竞态测试和 Windows amd64 交叉 vet。

## Results

- 原版的 Go 回归复现普通列表、分类页及 App 首页整页失败。修复后混合列表保留可解码的正常剧目，真实空页继续有效，全部无法解码或原始记录数超过上限则返回片源错误。含异常记录的详情返回片源错误，播放不签发票据。
- Go 全量、vet、CMS 与 App 竞态、Windows amd64 交叉 vet 通过。
- Apple Silicon 原生 Wails 隔离构建以合成网络响应验证首页、关键词搜索、分类和真实 MP4 解码。播放通过 Go 签发的环回票据。关闭进程后隔离 SQLite 的 `quick_check` 为 `ok`。原生探针首次链接缺少系统框架，显式链接 `UniformTypeIdentifiers` 后完成验收。

## Review Triage Log

- 分类页保留协议校验。`type_id` 的原值单独用于分类核验，不属于逐条 JSON 结构解码失败。已补混合页测试，确认正常行与不可核验分类 ID 同页时仍拒绝该页，符合保留分类校验的要求。逐条解码失败的行不作为分类数据使用。
