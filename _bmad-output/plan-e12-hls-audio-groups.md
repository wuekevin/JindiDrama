---
title: 'E12 HLS 音轨组成员名称及默认音轨校验'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '16b3226f45d4d77c287a19c210bb93c6444afedf'
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

在接受授权 HLS 主清单前，确认同一个音轨组内名称唯一且至多一个默认成员。允许不同组复用名称，保留原来的 URI 改写和 HTTPS 限制。合法普通话与粤语轨仍可分别取得自己的点播清单和分段。失败的主清单不公开任何子资源。

</frozen-after-approval>

## Implementation Notes

RFC 8216 §4.3.4.1.1 定义组内名称与默认成员约束。现有解析器只记录组存在。复用现有音轨组状态补上校验，并通过真实本机中继请求验证。

## Verification

- 红测试确认原版接受歧义清单。
- 合法同组成员通过，跨组可重用名称，各自 URI 保持绑定。无效主清单返回 502，子资源不可访问。
- Go 全量、vet、中继竞态和 Windows amd64 交叉编译。

## Results

- 原版对重名和多默认成员均返回 200。修复后返回 502 且余下子资源不可访问。不同组复用名称仍可通过。
- 中继验证普通话与粤语对应不同 URI 和完整分段。两个独立 AAC 合成分段经中继后逐字节一致，下载的 MPEG-TS 解码分别测得 440 Hz 和 660 Hz。FFmpeg 的直接 HLS 模式拒绝不带扩展名的中继路径，改为从中继取回实际分段后独立解码，未调整应用的中继地址或媒体限制。
- Go 全量、vet、HLS 与媒体中继专项及竞态、Windows amd64 交叉编译通过。Chromium HLS 专项 2 项通过。

## Review Triage Log

- 快审复核了音轨组边界、清单失败后资源不发布、测试证据与 HTTPS 约束，未发现需要修改的问题。
