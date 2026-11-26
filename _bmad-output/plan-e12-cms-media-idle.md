---
title: 'E12 苹果 CMS 媒体转发空闲超时'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '9ae6243c15bb912f60e2fa98d22245156388be69'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
followup_review_recommended: false
context: []
warnings: []
deferred: []
---

<intent-contract>

## Intent

上游媒体在发送响应头后停滞时，连接可能保持到 15 分钟播放票据失效，用户要等很久才能重新播放。对每次媒体响应体的等待设定空闲上限，让已经开始的截断流以失败结束，未发送的 HLS 清单返回错误。票据仍可用于同集备用源或重试，持续传输的媒体不受总时长限制。保留既有地址验证、范围请求与销票取消。

</intent-contract>

## Implementation Notes

转发和异常退出集中在一个 Go 服务，使用单步路线。通过 HTTP 请求上下文取消真实上游的读取。媒体 HTTP/1.1 使用 TLS 底层读期限，连接复用时会刷新期限。HTTP/2 使用单独响应体计时，避免在共享连接上干扰其他流。票据截止时间继续负责断开阻塞的下游写入。播放票据不会因为一条上游请求空闲而作废。

## Plan Change Log

## Review Triage Log

### 2026-11-27 — Review pass

- verdicts: 1 finding — high 0, medium 1, low 0, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` HTTP/1.1 分块解码可能在一次正文读取中先拿到数据、再等待下一块正文，使仍在传输的流误超时。真实 TLS 夹具复现首版提前截断。按底层读取刷新 HTTP/1.1 空闲期限，连接复用时重新计时。HTTP/2 保留单响应超时。两个协议以及分开发送分块长度与正文的场景均通过复验。

## Verification

- 使用真实本机 TLS 上游和本机转发，确认响应体停滞会取消上游，截断视频请求失败，HLS 清单返回错误，票据可读备用源，持续传输超过单次空闲上限仍成功。
- 运行 Go 全量、vet、媒体与 HLS 竞态回归和 Windows amd64 交叉构建，保留接口与失败复现记录。
- 本机 HTTPS 回归复现旧行为，修复后重复五次通过。Go vet、全量 Go、针对 Catalog 和 Apple CMS 的 race 及 Windows amd64 交叉构建通过。

## Auto Run Result

- `catalog_media.go` 执行每次响应的过期和错误中断，媒体正文完整后才允许流正常结束。`catalog_media_transport.go` 对协商到 HTTP/1.1 的媒体连接启用逐次 TLS 底层读取期限，HTTP/2 中的单响应继续通过请求上下文取消。
- `catalog_media_test.go` 用真实的本机 HTTPS 验证停滞媒体、停滞 HLS 清单、同票据备用源、慢速正常传输、可复用的 HTTP/1.1 连接和拆分的分块长度与载荷。
- 快审修复 1 个 medium。HTTP/1.1 原地等待后续分块时，以底层传输而非一次解码调用的整个耗时判断空闲。无延期项。
- 最终双协议回归重复 5 次、Go vet、Go 全量、Catalog 与 Apple CMS 竞态测试、Windows amd64 生产交叉构建通过。真实公网端点和原生 Windows/Intel Mac 仍待实测。未触发 GitHub 打包。
