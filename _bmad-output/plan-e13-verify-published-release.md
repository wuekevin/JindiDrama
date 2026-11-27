---
title: 'E13 正式发布后复核交付包'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '8cab7fdfe469b6dcf07b0677e00abf731caeadae'
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

提供发布后的只读校验入口。给定 GitHub 的公开版本元数据和已验收交付包，核对发布状态、频道、远程标签、版本页、下载地址、附件摘要及签名打包任务。错误版本、草稿、丢失附件和不匹配地址须被识别。保留现有发布前草稿验证入口。

</frozen-after-approval>

## Implementation Notes

复用草稿校验、标签检查及原始 ZIP 检查。发布后增加公开元数据的时间、版本页和附件地址验证。正式客户端发版后的实际安装包下载另验。

## Plan Change Log

## Review Triage Log

- 已修复。原始时间解析会接受年份或自动修正不合法日期。现在要求 GitHub UTC 时间戳，并按真实日历往返比较。

## Verification

- 用同轮合成交付包和远程标签夹具验收已发布版本及地址、附件和状态反例。通过既有草稿用例。

- 13 项 Node 发行脚本测试及 .54 双平台原清单通过。合成公开版本通过全流程复核。
- 项目公开列表为空。正式发布后的发现、下载、校验和安装仍待跑通。
