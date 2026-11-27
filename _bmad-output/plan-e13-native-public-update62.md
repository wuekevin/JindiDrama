---
title: 'E13 .62 原包的公开更新渠道和联网恢复验收'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '1d96d80b2ab97af07c35bd725dc12548a9e800b7'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: [_bmad-output/plan-e13-local-candidate62-packages.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

在 .62 原始 Apple Silicon 测试包的真实设置页验收更新渠道。先确认隔离网络故障有错误反馈，再让同一进程用真实 GitHub 接口重试，验证稳定版与预发布版结果、保存渠道后的重启和本机状态。GitHub 当前无可下载正式包时，验收仅覆盖联网、发现状态与恢复。

</frozen-after-approval>

## Implementation Notes

使用本机独立 SQLite 目录和 .62 原包。隔离代理只接受指定 GitHub 域名的 CONNECT，在故障阶段拒绝连接，恢复时转发原始 TLS 流量。原包出现上轮失败和本轮成功同时展示的问题。只清除由上一次版本查询产生的提示，保留安装包完整性告警。

## Verification

- 最终归档、应用进程版本架构和实际主程序摘要相符。
- 独立核对 GitHub 两个响应并验证原生应用同进程恢复、渠道换选以及重启。

## Results

- .62 归档通过 SHA-256 并从原二进制运行。仅拒绝 CONNECT 的隔离代理触发了桌面错误。恢复相同进程的正常 TLS 流量后，从 GitHub 得到无稳定版及预发布列表为空。原包在成功结果出现时仍显示上次失败。
- 设置页只跟踪版本检查提示。新检查开始或更新渠道保存完成时，撤下上一次结果。下载校验警告用已有独立入口处理。
- BMad 验收使用有独立标识的 Apple Silicon 临时签名构建复验，内嵌文件与生产前端逐项匹配。复跑通过同进程失败恢复、切换渠道时撤下上一次结果、预发布设置入库并重启保留。辅助原生构建补上链接框架并使用新的 WebView 测试配置，原始 .62 包保持不变。
- Chromium 设置页 45 项、WebKit 更新专项 4 项、Vitest 125 项、类型检查与生产构建通过。GitHub 当前无正式或预发布安装包，在线签名升级待实际发布环境。


## Review Triage Log

- low，已补齐。发行源码检出中的证据位置使用主工作区目录说明，主工作区保留可点击的证据链接。
- low，已补齐。冻结提示计时并暂停重试请求，覆盖请求尚在进行时清除旧提示。下载回归另外确认重新检查更新仍保留完整性警告。
- low，已补齐。在 1448×1086 使用相同的浏览器夹具对照修复前后，旧版同屏两条提示，修复版只显示当前结果。
