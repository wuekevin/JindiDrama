---
title: 'E13 GitHub 测试双平台归档复验'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'f1cc14ac223516652634f84ce37499ccd7674b86'
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

为累计测试包提供一个双平台本地复验命令。给定预期版本、仓库、提交、运行和轮次，命令核对 Windows 和 macOS 的 GitHub 原始归档、归档内清单、解包文件和来源。任一平台不匹配时返回失败。

</frozen-after-approval>

## Implementation Notes

复用既有包清单、GitHub 来源核对和 Python ZIP 比较。正式交付的现有参数与校验保持兼容。

## Verification

执行打包脚本测试，再使用归档留存的 .52 实际 GitHub 原始 ZIP、元数据与双平台解包文件，验证正确轮次通过、错误轮次失败。

## Review Triage Log

- 中等。两平台的元数据可重复声明同一个 GitHub 产物 ID，已在两平台核验时拒绝重复，并加入回归。
- 低。把真实归档、错误轮次和脚本测试的最终结果写入发行状态。

## Results

- 真实 .52 Actions 原始归档和元数据复验通过，两平台清单共 13 项，两个清单文件也与原始 ZIP 内的摘要匹配。
- 把同一份归档当作第 2 轮时被拒绝。Node 22 的打包和正式发行回归共 17 项通过，包含混合平台轮次、重复产物 ID 与原始 ZIP 字节变化。
- 证据使用已保存的 GitHub 产物。本轮未触发新打包。
