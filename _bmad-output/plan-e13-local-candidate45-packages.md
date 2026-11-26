---
title: '.45 累计候选本地打包验收'
type: chore
created: '2026-11-27'
status: built
baseline_revision: 523cc8b8b38b698e18e849b69dca96212de290ed
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

将 .44 后的 Windows CMS 协议验收、CMS 同类短剧、Windows 更新保存保护和版本说明全文入口合在一份本地候选。验证隔离发行源码、双平台成品和 SHA-256 清单，记录上线仍需完成的实际验收。

</frozen-after-approval>

## Implementation Notes

从独立发行源码打包。复用与该提交前端目录一致的生产构建，并逐文件核对浏览器回归与包内输入。使用同一候选版本号构建 Windows x64 和 macOS 双架构测试包。

## Verification

- Go、前端单测、完整 Playwright 回归与打包脚本测试。
- 验证两平台 13 项 SHA-256、实际安装器内容、macOS 架构和最低系统版本。
- 从最终 macOS 归档原生启动、退出、检查 SQLite 和回滚。

## 实际验收

- 首轮全量有一个备用视频位置断言时序失败。固定恢复后等待手动播放的状态，专项三轮 12 项通过。最终按 `317f4bfa592a22dc3ed6465e24e70b5ca0e4ccb2` 重建并通过 384 项全量回归。
- 两次构建的生产源码一致。最终包的前端生产 SHA-256 与回归输入一致。Windows 与 macOS 13 项文件校验及归档复核通过。
- Go 全量与专项竞态、122 项 Vitest、发行脚本、Apple Silicon 解包启动和回滚通过。Windows 原生协议、更新路径及 Intel 启动待运行。
