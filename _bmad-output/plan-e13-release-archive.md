---
title: '发行草稿核验 GitHub 原始交付 ZIP'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 8cccf5d5f0ab0d7990248abf073f8a4f97958f0d
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

正式发行核验应把草稿资产绑定到 GitHub 成功打包轮次实际上传的原始交付 ZIP。对照制品元数据确认名称、轮次、提交、字节数和外层摘要，并核对 ZIP 内每项文件与准备发布的文件一致。

</frozen-after-approval>

## Implementation Notes

复用现有草稿校验的资产名、文件长度和摘要。用 Python 标准库对已经核对摘要的 ZIP 逐项读取，检查文件集合、长度和摘要。README 记录按 Artifact ID 获取原始 ZIP 和元数据的命令。

## Verification

- Node 22 与 26 使用合成 Release ZIP 验证成功、错误 GitHub 元数据、ZIP 损坏，以及改变成员后重新计算外层摘要的拒绝路径。
- 使用已有 GitHub smoke 原始产物和归档文件验证 ZIP 读取及外层摘要，正式草稿继续使用合成数据。


## 实际验收

- Node 22 与 26 各通过 13 项发行工具测试，包含外层 ZIP 破损、摘要被重算后内层文件被替换、清单缺失和 GitHub 制品归属错误。
- 从 GitHub API 只读获取 .44 原始产物元数据。Windows 和 macOS ZIP 的字节数和 SHA-256 匹配。档内的 15 项成员和现有归档一致，包含两份清单。正式签名交付与线上草稿仍待正式发行环境验证。
