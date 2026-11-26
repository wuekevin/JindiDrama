---
title: '发行草稿核对签名打包轮次'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: e5ac797aefad6d855fa76ca8721dd4d570a8a516
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

正式发行草稿校验目前依赖维护者人工核对打包运行。校验时应对照 GitHub 指定运行轮次的任务响应，确认签名打包、跨平台下载复验和交付包组装均已通过。

</frozen-after-approval>

## Implementation Notes

复用交付说明已核验的仓库、运行编号、轮次和提交。草稿校验额外读取 Actions 任务响应。签名的 Windows 与 macOS 作业，以及 Windows 下载复验、Intel 启动复验和组装作业都须成功。签名矩阵的显示名称显式固定在工作流中。README 记录查询命令。

## Verification

- Node 发行工具回归覆盖正确轮次、混用轮次、错误仓库或提交、任务重复、缺失、失败或跳过。保留既有草稿、标签与资产校验。
- 只读查询现有 smoke 的 GitHub 任务响应，核对字段和测试批作业名称。


## 实际验收

- Node 22.23.5 与 26.8.2 各通过 13 项发行工具测试。实际 .44 GitHub 响应包含预期运行、轮次、提交和任务状态字段，组装作业为跳过。
- 打包 YAML 语法通过。真实签名打包任务、正式交付包和 GitHub Release 草稿仍待发行环境联调。
