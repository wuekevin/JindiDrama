---
title: 'E13 播放恢复与发行验收批次'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'fa39b56d9e78784a033507866c56d337abd99423'
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

将 .41 后的发行标签与 macOS 正式包分发检查、收藏与历史重试、备用源继续播放和停滞恢复在同一隔离源码上验证。使用同一前端生产构建生成双平台本地 smoke 并运行全量回归。按实际执行环境记录原生、签名、授权片源与 GitHub 验收范围。

</frozen-after-approval>

## Implementation Notes

本地构建与浏览器回归使用独立检出。发行产物核对 SHA-256、资源内容和平台架构。正式验收与本地 smoke 证据分别记录。

## Plan Change Log

## Review Triage Log

## Verification

- 在同一源码和前端哈希上完成双平台构建、浏览器与 Go 全量测试、发行辅助工具回归及漏洞扫描。
- 校验双平台清单、安装器成员、macOS 两个架构和本机 Apple Silicon 启动。

## Auto Run Result

- 交付 .42 本地 Windows 和 macOS smoke。两套清单共 13 项 SHA-256 复验通过，安装器和切片结构检查通过。
- 同生产构建通过 374 项 Playwright、122 项 Vitest、Go 全量及发行辅助回归；Apple Silicon 从最终归档正常启动退出，独立 SQLite 完整。Go 与 npm 生产漏洞扫描均无报告。
- 真实授权片源、Windows 与 Intel Mac 当前批次原生、正式签名与公证仍待验收。
