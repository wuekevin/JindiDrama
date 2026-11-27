---
title: 'E13 Wails 开发依赖 x/crypto 更新'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'e27173a4fec37734cfbaf056d729ddcca3b5f29a'
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

清理当前 Go 开发构建扫描中的 x/crypto 模块级告警。升级到已移除旧 OpenPGP 包的正式版本，验证正常依赖解析和生产构建范围。

</frozen-after-approval>

## Implementation Notes

从实际开发和生产构建的包清单定位 x/crypto。只调整 Go 依赖和对应完整性摘要。既有 .58 归档继续对应原始源码。

## Verification

核对开发和三种生产目标的 Go 依赖，运行 Go 全量、vet、模块校验与开发及生产标签扫描。实际启动 Apple Silicon 的开发程序，验证环回资源与正常退出。

## Results

- v0.59.0 只变更 x/crypto 和对应摘要。开发仍导入 ACME，生产包清单和对应构建信息没有链接该模块。
- 两种构建标签的漏洞扫描均报告无漏洞，Go 全量测试、vet、模块完整性检查均通过。
- Apple Silicon 开发程序实际启动、提供本地前端资源并正常退出。Apple Silicon 生产归档通过实际窗口、正常退出和活动 WAL 回滚。Windows、Intel Mac 核对生产目标依赖。
