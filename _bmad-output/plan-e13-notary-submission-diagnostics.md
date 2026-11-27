---
title: 'E13 发布日志保留 Apple 公证提交编号'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '5da52e1fa172e05fa32813aa7fa20368bf94ac92'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

正式 macOS 打包在 notarytool 返回提交编号时保留可查询的编号，包括拒绝及命令失败。只有成功退出且明确 Accepted 的提交才继续装订。日志不输出原始 JSON，保留原有临时文件清理和取消处理。

</frozen-after-approval>

## Implementation Notes

复用原有 JSON 结果文件和 macOS plutil。只输出符合 UUID 文本格式的编号和已知状态，记录命令非零退出码。README 说明如何使用维护者受保护的 Keychain 配置取回公证日志。

## Verification

- 使用现有签名夹具和本机 macOS plutil 复现编号丢失，验收接受、拒绝、非零退出、缺少编号或恶意文本及取消后的行为。
- 运行 macOS 工具全量、Bash 语法与发行脚本测试，确保临时目录清空、拒绝后未装订。

## Results

- 原版夹具复现成功、拒绝与非零退出都丢失有效提交编号。修复版在结构化响应中读取安全的编号与已知状态。保留非零退出码，装订继续要求工具成功和准确的 Accepted 状态。拒绝、取消及文件清理回归通过。
- Bash 语法、签名夹具 11 项、macOS 工具 18 项及 Node 发行脚本 17 项通过。Apple Xcode 本机帮助核对 `notarytool log` 的 UUID 参数和 Keychain 选项。实际 Apple 账户提交及公证待正式凭据。

## Review Triage Log

- 已修正字段精确校验。Bash 会移除命令替换末尾的换行。读取时将不允许的字节转换为非法字符，再检验编号和状态。新增编号和状态末尾换行的回归。
- 已覆盖返回 Accepted 但工具退出失败的响应。同时确认编号仍被记录、原退出码保留且未执行装订。
