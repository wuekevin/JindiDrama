---
title: 'E13 默认分支同步演练'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'ac183c0b225d6b2dbc114e0ba3e04f7a62205356'
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

在本地验证把经过验收的最新发行源码同步给旧 GitHub 默认分支的方式。保留本地 main 两个提交和既有历史。验证审核候选与独立发行源码文件树一致，正式打包继续使用独立历史。

</frozen-after-approval>

## Implementation Notes

从当前远端基线和本地 main 核对提交关系。使用隔离检出生成同时保留两段历史的本地候选，候选使用独立发行源码的精确文件树。在保存本轮计划和状态之前核对原工作区及暂存区。已测试的候选提交和它的父提交保留原样。

## Verification

核对 Git 祖先关系、文件树字节标识和发布历史检查。运行前端构建、单测、Chromium 全量、WebKit 切源专项、Go 全量、vet、打包脚本及 Go 开发与生产扫描。复核路径和当前工作区变更。

## Results

- 核对本地两条提交、远端默认分支基线、独立发行提交均为候选祖先，候选和独立发行提交的工作树完全一致。候选保留于本地审核分支，远端读回确认 main 仍在基线，且无同名审核分支。
- 候选的依赖锁定避开 35 项已核对告警的所有受影响版本范围。Go 两种构建标签和 npm 生产扫描均未报告告警。
- Chromium 401 项、WebKit 5 项、Vitest 125 项、Go 全量与 vet、发行脚本 17 项通过。生产前端摘要与独立发行检出一致。
- 旧默认分支历史不能通过现有正式发行源码检查，独立发行历史通过。保存本轮计划和状态前，主工作区与暂存区的差异和状态摘要均与演练前相同。

## Review Triage Log

- 低。远端证据限定于 main 和同名审核分支，状态改为记录实际读回的范围。
- 低。说明验收日志位于本机主工作区的生成物目录，独立克隆不包含这些日志。
- 低。把工作区校验明确标为验收文档落盘之前。落盘只同步计划和状态，审核分支仍指向经过验收的原提交。
