---
title: 'E12 苹果 CMS 拒绝重复 JSON 字段'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '96ae13cc49245dd8280ccf34d68d2e2ee145efed'
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

当 CMS 响应或剧目记录的 JSON 字段重复时，禁止将重复字段处理为可确认的目录、详情或播放权限。目录仍保留正常记录，详情和播放对歧义响应返回片源错误。单独使用的原有字段大小写写法、分类核验和响应限额继续有效。

</frozen-after-approval>

## Implementation Notes

在响应对象、剧目和分类行各自的边界检查字段唯一性，匹配现有 Go JSON 解码的大小写折叠。浏览和精确详情复用现有错误处理。

## Verification

- 先复现再运行 Provider 与 App 回归，包括重复响应字段、记录字段大小写变体、分类行和正常兼容输入。
- 运行 Go 全量、vet、App/CMS 竞态和 Windows amd64 交叉 vet。确认有歧义的详情请求没有媒体票据。

## Results

- 原版 Provider 用例复现重复响应字段、上架字段和权限字段被解码，并显示重复字段的目录或分类行。修复版拒绝有歧义的响应和精确详情，目录保留正常行。专项覆盖 ASCII 大小写、Unicode 折叠和 JSON 转义后同名字段。
- Go 全量、vet、App/CMS 竞态和 Windows amd64 交叉 vet 通过。
- Apple Silicon 的隔离 Wails 构建使用合成片源数据运行收藏和历史两个会话。重复字段响应显示重试，修复夹具后在原页面继续观看，保留原本的 25% 进度与 6000 ms 断点，正常关闭和 SQLite 完整性通过。

## Review Triage Log

- 低，已补。桌面 App 集成夹具为同一剧目保留有效媒体地址，增加重复上架字段时的详情错误、拒发票据和零媒体请求断言。正常响应仍走既有真实 Go 环回播放验证。
