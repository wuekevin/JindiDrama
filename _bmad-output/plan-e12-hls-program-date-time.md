---
title: 'E12 授权 HLS 点播时间戳兼容'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '2c43e112d5836b59fa1d0157506e88fba5eb27e9'
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

授权 HLS 点播清单带有节目时间戳时仍可播放。验证常规 ISO 日期时间字段，将其保留在所对应的分段前。畸形值、无对应分段或混入主清单时继续拒绝整个清单，子资源仍由 Go 中继管控。

</frozen-after-approval>

## Implementation Notes

复用清单的逐行状态和媒体清单角色。RFC 8216 §4.3.2.6 的时间戳只应用于下一分段。用本地中继覆盖清单和分段请求。

## Verification

- 原版回归复现，Go 中继与 HLS 专项，全量测试和 vet。
- FFmpeg 生成带节目时间戳的点播素材，读取中继输出后解码实际分段。

## Results

- 原版回归返回 502。修复后，授权清单及其子资源通过中继。两段本机 FFmpeg 点播素材原样取回，各解码 30 帧。
- Go 全量、vet、HLS 与媒体中继竞态检查、Chromium HLS 5 项通过。Chromium 和 WebKit 均实测分钟精度、时区偏移及小数分隔符解析。
- 本机解码复跑时相对路径不能从 Go 测试进程读取素材，改用绝对路径后复验。

## Review Triage Log

- 中，已修复。分钟精度是有效 ISO 8601 格式。中继接受并保留，新增分段回归覆盖。
- 中，已修复。Chromium 与 WebKit 的 Date.parse 均拒绝逗号小数。中继改用小数点，保留所表达的时间。回归测试核对输出。
- 低，已修复。将原版复现、真实分段解码和浏览器检查结果写入 docs/STATUS.md。
