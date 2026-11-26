---
title: 'E13 当前发行门槛和签名凭据说明'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'cf43a85411f5b454b717ded59f5762bd426b62ce'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

让发行维护者能从 STATUS.md 开头找到当前候选、各项验收证据和正式发行所缺输入。README 说明 Windows 托管签名对签发方允许的 PFX 工作流的依赖，以及已有硬件密钥或不可导出证书的本地签名覆盖范围。

</frozen-after-approval>

## Implementation Notes

发布工作流按现有凭据要求运行。文档按 GitHub 实际运行和配置、签发方公开规则以及现有 Windows 证书库脚本核对。

- 两平台共 13 项本地摘要重新通过，核对了 Apple Silicon 成品及原生下载日志。GitHub 查询确认没有正式 Release、签名环境和仓库级 Actions secrets。当前发行说明区分旧版托管证据和 .49 本地证据。
- README 按 CA/Browser Forum 基线和当前 GitHub 工作流核对证书输入。保留硬件证书库的现有路径，记录托管正式交付所需的签名验收。

## Plan Change Log

## Review Triage Log

## Verification

- 重验 .49 本地两份清单和 Apple Silicon 原生验收日志。查询 GitHub 上一轮原生任务及正式环境、仓库级 secrets、Release 数量。
- 从当前提交读取 Windows 和 macOS 工作流配置、Windows 本地签名说明及上游授权门槛。
- 验证文档修改、链接和发行源码历史，保护现有工作区改动。
