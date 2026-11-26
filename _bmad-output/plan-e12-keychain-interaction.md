---
title: '限制 macOS 钥匙串操作的系统交互'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 28b2fd5d2a28feb160922f924bbfc6ac793b6791
route: oneshot
route_source: pinned
risk: medium
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

macOS 既有文件型钥匙串的读取、保存和删除要以明确的非交互方式执行。权限不足时返回凭据库不可用，操作后恢复进程原来的钥匙串交互状态。

</frozen-after-approval>

## Implementation Notes

保留现有服务和账号命名及默认钥匙串。只在很短的系统调用阶段关闭文件型钥匙串交互，在互斥范围内恢复进程状态。保留查询参数的交互约束。

## Verification

- Darwin 原生测试检查两种原有进程状态、操作中关闭和操作后恢复。
- 使用随机合成凭据测试现有访问、替换、删除和配置目录隔离。
- Go 全量与凭据库竞态、Windows amd64 交叉编译。

## 实际验收

- 在 Apple Silicon 系统原生 API 上，两个原始交互状态都能在单次操作后恢复。本地测试过程使用随机合成凭据并清理临时项。
- Go 全量、Go vet、原生凭据竞态、无 CGO 路径、Windows x64 凭据测试及生产应用交叉编译通过。
- 受锁钥匙串和签名版跨版本访问待实际运行环境验收。
