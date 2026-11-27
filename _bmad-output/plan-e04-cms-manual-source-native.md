---
title: 'E04 苹果 CMS 手动切源原生验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '0e83db98ac84c022091cc3b505df57fd199c9f13'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

把苹果 CMS 手动切源从浏览器夹具验收到 Apple Silicon 桌面。给定两条可播放的线路和一条返回 404 的线路，WKWebView 应能在暂停时恢复断点，在故障回退后继续播放，并在正常退出时保留观看进度。

</frozen-after-approval>

## Implementation Notes

复用原生 CMS 测试构建的隔离数据目录与 Go 传输夹具。保留 Wails 绑定、Go Provider、媒体租约与代理、生产前端和 SQLite。

## Verification

核对嵌入前端摘要、构建架构、代理媒体请求、WKWebView 的已解码视频、暂停和恢复后的时间、故障线路状态、正常退出及 SQLite 断点。

## Results

- Apple Silicon macOS 26.6.2 的真实 WKWebView 从 Go 环回地址读取两条有效线路。暂停切源后保留 7.000 秒，且观看历史为空。播放时选中返回 404 的线路，回退到第一条线路并保持静音和播放。
- 同一媒体票据的渠道编号按 0、1、2、0 变化，原生辅助功能报告「播放源」和选中的「线路 1」。保持播放并触发原生退出前 SQLite 的已有断点为 7.250 秒，退出后更新为 10.080 秒。SQLite 完整性检查通过，辅助文件和会话标记已清理。
- 首轮脚本把预读元数据的暂停视频也要求达到播放数据就绪，后来改为在暂停阶段验证元数据与定位，在播放阶段验证解码与时间推进。第二轮恢复播放后仍未展开辅助功能树。第三轮先激活 Wails 窗口后复验辅助功能树通过。

## Review Triage Log

- 中等。首份证据在关闭前主动暂停，已有暂停写入可以掩盖退出时未更新断点。复验保持播放，在触发系统退出前读出旧断点并立即释放只读连接，退出后检查新的断点及 SQLite 状态。
- 中等。构建前后目录 SHA-256 相等无法直接验证二进制内的内容。逐项扫描成品 Mach-O，对 69 个非空前端文件进行字节匹配及 SHA-256 比对。
- 低。修正验收日志对应的轮次。运行证据放在项目本机的生成物目录。
