---
title: 'E12 苹果 CMS 无效上架状态支持重试'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'a8116f2bd0464327c90bbe35a98a86009916fb79'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

详情响应中剧目的上架状态无效时返回可重试的上游错误，收藏与历史可逐条重试核验。明确下架的剧目维持未找到，兼容只提供上架目录而省略状态的旧片源。无效与下架状态均不得签发新播放票据。

</frozen-after-approval>

## Implementation Notes

Go Provider 的目录和详情共用状态读取。目录继续只展示可确认上架的条目。详情将明确下架和无法确定状态交给现有的未找到和片源错误路径。

## Verification

- Go Provider 与 App 复现原版的错误分类，核验数字或字符串的下架状态、状态省略、异常值及播放拦截。
- 编译生产前端并运行收藏与历史重试和真实下架的浏览器回归。运行 Go 全量、vet、CMS 与 App 竞态。

## Results

- 原版 Go 验证复现 `null`、对象及未识别数字均被归为未找到。修复版通过 Provider 和 App 回归，异常详情可重试，明确下架返回未找到，播放拦截不产生媒体请求。Go 全量、vet 和 App/CMS 竞态通过。
- 生产前端构建后 Chromium 与 WebKit 分别通过 4 项收藏、历史和片源可用性回归。
- Apple Silicon 原生 Wails 隔离构建使用合成片源和两个独立 SQLite 资料。收藏和历史各保留 25% 进度并在恢复正常状态后原页重试。两个会话正常退出，数据库完整性以及原始 6000 ms 断点核对通过。

## Review Triage Log

- 本次状态分类通过快审。另记待办：现有 Go JSON 解码对同一记录中的重复上架字段使用后值。后续应验证并拒绝会影响上架、权限或播放地址判断的冲突字段。
