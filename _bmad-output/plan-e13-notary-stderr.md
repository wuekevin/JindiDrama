---
title: 'E13 macOS 公证日志输出边界'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: 'abfb6ae8dd14fb671dd32341d0c7e542ad9dbe98'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

正式 macOS 公证的工作流日志仅记录已校验的提交编号、已知状态与命令退出码。工具返回在标准错误的未经校验文本不得进入日志。保留状态门槛、取消和清理，以及通过 Apple 提交编号进行排查的能力。

</frozen-after-approval>

## Implementation Notes

现有脚本在私有临时目录保存 JSON，仅读取提交编号和状态。子进程标准错误仍直连 GitHub 日志。沿用结构化诊断，并将正式公证阶段的工具输出替换为固定阶段名和退出码。

## Verification

- 使用脱机合成工具输出，在退出成功、失败和取消时验证日志边界、退出码和清理。原版须能复现合成文本进入日志。
- 用 macOS Bash 语法检查和现有完整公证脚本测试覆盖。

## Results

- 原版在成功、失败和取消三种情况下都将合成诊断文本写进进程日志。修复后，三种路径继续保留预期退出行为，日志只记录已校验的结构化字段和固定阶段的失败退出码。追加覆盖身份检查、归档和票据装订的错误路径。
- macOS 系统 Bash 语法检查、Python 公证与桌面冒烟 19 项、Node 交付与校验 18 项、源码历史校验夹具通过。
- 本机系统 codesign 拒绝了临时签名应用进入正式公证，退出码为 3。临时目录已清理。整批测试使用合成凭据。

## Review Triage Log

- 中，已修复。初版只隔离 notarytool 输出，代码签名检查、打包与装订仍直通。整步改用固定阶段诊断。覆盖工具 stdout 和 stderr，保留原退出码。回归复跑通过。
