---
title: '截断的版本说明提供全文入口'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 6f13603042818faf396cd8788f3d88a0b1f25541
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

当桌面更新日志超过本地展示长度时，告知用户当前展示的是部分说明，并可从弹窗打开本次检查确认的发布页查看全文。

</frozen-after-approval>

## Implementation Notes

Go 按字符截取内容并在发生截断时标记结果。Vue 保留纯文本渲染，全文入口复用 Go 的发布页检查和浏览器打开流程。

## Verification

- Go 测试覆盖长度边界、多字节字符和无更新响应。
- Chromium 验证纯文本、键盘焦点、已核验发布页链接，切换渠道撤下旧入口。
- 检查默认和 220% 缩放下的更新说明弹窗。

## 实际验收

- 更新单测覆盖完整边界、超过展示限制的多字节字符和无更新结果。Go 全量与专项竞态通过。
- 122 项 Vitest 和 44 项设置 Playwright 通过。验证当前版本 URL、按键操作、渠道切换和空正文。
- 默认及 220% 截图复核通过。Windows x64 生产程序成功交叉编译。
