---
title: 'Windows 正式成品绑定卸载器签名回执'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 7bfc44cca1cd5446754d64beadf266934c68dd63
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

正式 Windows 安装验收应确认卸载器就是同轮 NSIS 钩子签署并记录的文件。安装前复核最终签名的安装器和便携程序，安装、升级和重装后核对卸载器的 SHA-256、发布者与时间戳。

</frozen-after-approval>

## Implementation Notes

复用签名钩子产出的摘要和现有 SignTool 核验。签名脚本通过可选输出参数交给工作流，验收步骤需要该值。继续只交付既有安装器和便携文件。

## Verification

- PowerShell 夹具覆盖摘要匹配、字节变化、发布结果及失败清理。
- 检查 YAML 和工作流 PowerShell 语法。用本地 NSIS 钩子样例核对卸载器归档行为。


## 实际验收

- PowerShell 7.5.12 夹具验证匹配摘要可复验、字节变化会在 SignTool 前被拒绝、缺少回执时不发布，以及发布后返回签名钩子的准确摘要。打包分支在本机以模拟 Windows 外部签名工具运行。
- NSIS 3.13 隔离样例在卸载钩子中追加测试标记，7-Zip 提取的成品与钩子产物逐字节一致。另验证工作流 YAML、PowerShell AST 和 13 项发行工具测试。
- 原生 Windows 签名、实际安装、重装与升级仍待正式凭据和工作流验收。
