---
title: 'E13 公开发布附件实读校验'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '82e75f1c84bd2012737ec6ac3d2de51b55e6e0d0'
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

发布维护者可以在核对已公开的 GitHub Release、构建交付 ZIP 和远程标签后，无凭据读取每个附件并独立比对实际内容大小与 SHA-256。校验覆盖清单文件，拒绝无效重定向、截断、超限或字节不符。草稿及只读元数据交付复核仍可使用。记录无正式发布时真实验收的边界。

</frozen-after-approval>

## Implementation Notes

复用正式发布校验的既有附件集合、摘要及 API 地址校验。按桌面更新使用的公开资产下载路径验证，不需要本机保留安装包的额外副本。

## Verification

- 运行发行脚本测试，覆盖公开下载命令接线、正常附件、校验失败、响应长度及重定向限制。
- 对公开已有附件验证实际 HTTPS 下载链；对 JindiDrama 查询是否已有可测稳定发布。

- 本轮发行脚本 16 项通过。公开下载入口从测试的 11 个已验证附件读取实际字节；单独破坏清单响应即可阻断通过。
- 真实 GitHub jq 1.8.1 公开附件匿名读取、大小和 SHA-256 通过。JindiDrama 稳定入口为 404，公开发布列表为 0。

## Review Triage Log

- 已补齐 STATUS.md 的项目发布状态与真实下载验收范围。
- 快审发现 Node URL 抹去显式 443 端口。已在解析前校验重定向原始主机，确保该输入与桌面 Go 更新器一起拒绝。回归检查显式 443 与其他非预期位置，实际公开下载复跑通过。
