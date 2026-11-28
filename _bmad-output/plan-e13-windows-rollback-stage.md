---
title: 'E13 修复 Windows 成品的回滚暂存目录'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '53f0f535b2d21737e4b7053a4f869c66d5904d75'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

.67 安装版 WebView2 正常启动后，成品回滚因暂存目录权限拒绝而中止。修复 Windows 回滚暂存目录的创建方式，保留当前用户与 SYSTEM 专用的访问列表、同目录句柄发布、源库不变和退出清理。按相同候选范围集中复跑三种架构的桌面冒烟。

</frozen-after-approval>

## Implementation Notes

工作区和目标卷暂存都使用父目录句柄，在 Windows 创建子目录时提供受保护访问列表。创建后读回 DACL、核对目录身份，再允许写入数据库快照。复用备份写入已有的原生创建和权限回读实现。用同名目录验证独占创建。

## Verification

- Windows x64 交叉编译应用以及 App/SQLite 测试。Go 全量与 vet 在本机通过。
- GitHub 安装版与便携版实际完成回滚，检查旧版可读与原数据库不变。下载后复验 Windows 与 macOS 清单。
- Apple Silicon 和 Intel Mac 对同一运行的 macOS Universal 归档完成原生启动和回滚。

## Review Triage Log

- medium，必须用修复后的 Windows 原生包验证 DACL。现有失败日志只证明 .67 在 WebView2 启动后回滚失败，交叉编译通过。复跑结果单独写入状态页。

## Windows 备份恢复复验

- 首轮全量 CI 还报告关闭数据库和活动 WAL 两种备份恢复失败。Windows 暂存目录的清理句柄持有 DELETE。顶层 Go `os.OpenRoot` 的 Windows 打开调用未共享 DELETE。使用父目录上的相对 `OpenRoot`，允许清理句柄共存。保留暂存标记、目录身份核对和清理顺序。
- .67.1 在 Windows 安装版和便携版通过回滚、活动 WAL 与原库不变。Apple Silicon 和 Intel Mac 在同批通用包通过原生冒烟。备份恢复补丁通过 macOS 定向测试和 Windows x64 测试交叉编译，Windows 原生全量 CI 复验见状态记录。
