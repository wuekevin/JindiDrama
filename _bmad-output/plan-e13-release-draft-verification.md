---
title: 'E13 GitHub Release 草稿资产核验'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '7f43ed90995ad40dd2a2f314aee1608e19bc4007'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

维护者发布桌面安装包前，可以核对 GitHub 草稿上传的文件是否逐一对应已验证的 Windows 与 macOS 交付包。发布标签、预发布标记、资产状态、字节数与 SHA-256 有任一差异时，命令返回失败。

</frozen-after-approval>

## Implementation Notes

扩展现有校验器，复用本地标签和交付包核验，读取 GitHub REST 草稿元数据。README 记载只读查询和核验步骤。

## Plan Change Log

## Review Triage Log

- 中 · 核验过程中二次读取清单可能混用两份版本。清单解析、草稿摘要和长度均使用同一份已读取的字节。
- 低 · 两平台和清单摘要需直接覆盖。补复验 macOS 和校验清单的摘要篡改，并运行稳定版实际文件夹具。
- 低 · 将真实执行结果和远端未验收范围补录至 STATUS.md。

## Verification

- 运行发行工具 Node 测试。用本地仿真草稿验证全量文件、平台包、SHA-256 清单、摘要和渠道的正反例。

## Auto Run Result

- Node 22 与 Node 26 各 13 项发行脚本测试通过。
- .42 测试包原始 13 项 SHA-256 通过。模拟稳定版和候选版草稿分别核验 11 项，清单摘要变化时失败。
- GitHub 有效资产响应字段与夹具一致。JindiDrama 无草稿或公开 Release，本次使用本机夹具且未触发托管工作流。
