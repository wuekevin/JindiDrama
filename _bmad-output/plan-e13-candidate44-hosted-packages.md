---
title: '.44 累计候选托管打包验收'
type: chore
created: '2026-11-27'
status: built
baseline_revision: eba24ce66d8a0c1840ce3b58e357a407639a75be
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

把已授权的累计候选推到独立测试分支，一次集中运行 Windows、Apple Silicon 和 Intel Mac smoke，并下载产物核对 SHA-256。遇到可恢复故障时修复并重跑，把实际验证记录到 `docs/STATUS.md`。

</frozen-after-approval>

## Implementation Notes

- 从隔离发行源码使用完整历史通过发行路径检查。测试分支为 `codex/smoke-20261127-44`。工作流输入为 `smoke` 和 `v0.1.0-smoke.20261127.44`。
- GitHub 打包与推送 CI 分别为 `45878896269` 和 `45878898001`，两项起始源码一致。

- 打包运行的四个 smoke 任务首轮通过。Windows 核对旧程序被占用时的退出码、保留文件和重试升级，安装与便携程序均验证回滚。macOS 双架构从同一 Universal ZIP 验证启动和回滚。
- macOS Artifact 的一个分段下载不完整。重新请求该范围并与已有分段合并后，外层摘要匹配。打包无需重跑。

## Plan Change Log

## Review Triage Log

## Verification

- 查验 GitHub 运行的提交、分支、尝试编号及各原生步骤结果。
- 下载两个原始 Artifact ZIP 并核对 GitHub 摘要。以候选源码的校验器和独立 `shasum` 检查解包文件，再检验 macOS ZIP。
- 将原始产物、解包产物及验收证据归档至本机忽略目录。

## 实际验收

- 原始 Actions ZIP 两项 SHA-256 与 GitHub 一致。解包清单 Windows 7 项、macOS 6 项，以及发行说明的 provenance 全部匹配。移入 `build/bin/github-smoke-20261127.44` 后独立复核通过。
- Universal ZIP 通过解包校验。成品含 arm64 和 x86_64，两个切片的最低系统版本均为 macOS 13.0。
- 同提交的 Windows 和 macOS CI 各通过 383 项 Playwright，以及前端构建、Go vet、Go 全量测试和 Wails 构建。两平台通过苹果 CMS 续看及分享回归。
