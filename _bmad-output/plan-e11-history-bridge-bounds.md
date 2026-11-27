---
title: 'E11 观看历史数值精度'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '0093b8570d24b548e8b47460a6949e9dd8ca55ff'
route: oneshot
route_source: auto
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

恢复备份和保存观看历史时，拒绝超出桌面桥接精度或日期范围的记录，避免集数、进度和时间在 Vue 里变成不同的值。保持有效旧备份和正常保存兼容。批量保存与恢复遇到无效输入时保留现有收藏和历史。已经写入本地库的数据继续按原来的读法读取。

</frozen-after-approval>

## Implementation Notes

SQLite 的集数和时间使用整数。WebView 通过 JavaScript number 取值，播放进度还经过毫秒转秒和再舍入。保存和恢复应使用同一验证边界，写入继续保留现有的百分比裁剪和零时间补全规则。前端按展示值比较旧进度，收藏变更时保留 SQLite 原始毫秒。前端验证新变更才入队，旧队列重放继续处理有效的编辑与删除。

## Plan Change Log

## Review Triage Log

- medium，已修复。无效记录会让旧重试队列永久失败。前端在缓存前拦截新的无效更新，旧队列重放仅提交有效的历史更新，并保留有效删除和收藏改动。
- low，已处理。评审时状态记录还未写入，最终验收写入 STATUS.md 并附日志。
- false。JavaScript 能编码单个 2^53，邻接的 2^53+1 却与它混同。集数采用安全整数边界，避免此类映射歧义，播放进度还需要通过毫秒往返校验。

## Verification

- 给定超出精度的集数、进度或时长，或超出日期范围的时间，备份读取和单条保存应拒绝，批量写入应回滚收藏、历史和回执改动。
- 给定可精确表示的集数、经过秒换算仍能还原的毫秒及可显示时间，保存后可由备份带回原值。
- 给定已保存的旧进度存在秒换算舍入，当只改收藏时，桌面桥接省略旧进度，正常的新进度仍可保存。
- Go SQLite 专项、全量、vet、App 和 SQLite 竞态，Windows amd64 交叉编译，并用 JavaScript 验证边界样本。

- 类型检查、前端生产构建、125 项 Vitest、91 项 Chromium、Go 全量、vet、App 和 SQLite 竞态及 Windows amd64 SQLite 交叉编译通过。
