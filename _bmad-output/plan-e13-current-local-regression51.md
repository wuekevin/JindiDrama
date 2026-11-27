---
title: 'E13 最新发行源码集中回归'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'a2b442ff0e1cd80ad5a3cb5a4a3fd61cedf0647b'
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

对 .50 本地打包后包含首页键盘焦点修复的完整发行源码做集中回归。用锁定依赖生成送验版本，验证完整浏览器套件、Go 回归和发行源码校验，为同一提交的跨平台验收建立本地基线。

</frozen-after-approval>

## Implementation Notes

测试在独立检出运行，保留现有 .50 两平台归档。正式签名、当前候选的 GitHub 原生运行及授权公网接口单独按现有门槛确认。

- 汇总 393 项 Chromium、122 项 Vitest、WebKit 专项、Go 模块校验、vet、全量和四个包的竞态测试。生产 Go 与 npm 审计未报告已知漏洞。
- 同环境 1448×1086 首页按像素一致。核对 package.yml 启用、上次托管仍为 .44、当前预留分支未使用、签名环境和正式 Release 均为零。

## Plan Change Log

## Review Triage Log

## Verification

- Node 22 锁文件安装、类型检查、生产构建、Vitest、Chromium 全量与 WebKit 首页专项。
- Go vet、Go 全量与 App/诊断/苹果 CMS/更新竞态。发行源码历史检查。
- 下载并核对当前 GitHub 工作流状态和签名环境数量。当前候选没有授权前不产生公开分支或托管运行。
