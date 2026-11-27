---
title: 'E13 复验正式发行的关键执行步骤'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '0ad1d2636bd2c55588bb0253fcfb3312d5b6c01f'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

交付包验证要求 GitHub 同一打包轮次中现有的正式签名、公证、原生运行和产物校验步骤各执行一次且成功。即使任务整体状态为成功，缺少或跳过关键步骤的记录也应失败。沿用现有草稿及公开复验入口。

</frozen-after-approval>

## Implementation Notes

使用现有工作流步骤名称和 GitHub Jobs API 的步骤结论，先验证任务归属和构建身份，再验证步骤。

## Verification

- 运行发行脚本测试，复现任务成功但关键步骤被跳过、缺失、重复或顺序异常。
- 对照 GitHub 实际测试轮次返回的步骤、现有工作流定义和当前任务范围。

- 原校验接受关键步骤跳过的夹具，修复后 16 项发行脚本测试通过。五个任务的 17 项关键步骤与当前 YAML 的名称及顺序相符。
- 已通过公开 GitHub Jobs API 查看 .52 任务的成功、跳过及 step number。项目正式签名和公证工作流尚无运行证据。

## Review Triage Log

- 已补充 STATUS.md 记录缺失的验证结果及真实运行范围。下载的是只读任务记录，用于核对 API 数据结构和 smoke 跳过结论。
- 评审发现 macOS 最低系统版本和 ZIP 完整性检查遗漏。补查两步，并核对 Windows 构建及两个平台的依赖锁文件检查。部署目标缺失或 ZIP 步骤跳过均由端到端测试拒绝。
