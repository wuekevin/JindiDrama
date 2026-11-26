---
title: 'macOS 公证核对签名团队和本次提交结果'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: fb6b2255b3c7181605548e52d0f8113473f64aac
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

正式 macOS 发行包送交公证前应验证当前 Developer ID Application 签名属于配置的 Apple 团队。本次公证提交应明确被接受才进入票据附加和校验。

</frozen-after-approval>

## Implementation Notes

保留签名与公证步骤的凭据范围。使用 codesign 要求表达式和 notarytool 的 JSON 响应。响应留在私有临时目录并纳入既有清理。继续使用现有取消处理和票据验证。

## Verification

- 本机验证要求表达式能拒绝已存档的 ad-hoc 签名。
- 签名脚本夹具覆盖团队不符、格式无效、进程成功但状态无效、已接受、取消与清理。


## 实际验收

- 旧脚本在模拟工具退出成功但状态无效时会继续完成。修复后确认拒绝 Invalid、进行中、状态缺失、类型错误和截断响应，临时文件均被清理。
- 本机真实 codesign 对已有 Developer ID 签名应用验证团队匹配通过，错误团队失败。项目 .47 ad-hoc 应用在联系 Apple 之前被新脚本拒绝。
- macOS 签名与成品工具 16 项 Python 测试、Node 发行工具 13 项及 Bash 语法检查通过。实际 Apple 公证仍需正式发行凭据和签名包。
