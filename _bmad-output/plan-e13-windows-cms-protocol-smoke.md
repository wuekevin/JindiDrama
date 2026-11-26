---
title: 'Windows 安装包的 CMS 分享协议冒烟'
type: chore
created: '2026-11-27'
status: built
baseline_revision: 9258b54cddcdb38abb239f21375ad52e02347a1f
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

为后续合并打包补齐 Windows 安装版的原生 CMS 分享协议验收。验证安装器实际注册的协议命令、系统 Shell 激活、同一运行实例恢复和卸载清理。

</frozen-after-approval>

## Implementation Notes

- 复用现有关闭到任务栏的冒烟隔离环境和 Win32 窗口观察。用固定的合成 CMS ID 核验协议转发，详情页渲染沿用已经通过的 Windows 浏览器回归。
- 工作流保留普通二次启动覆盖，并在已有安装和卸载步骤中检查协议生命周期。

## Verification

- 在本机运行 PowerShell 语法检查和会话测试，并检查两个共享打包分支的 YAML。
- 运行应用深链相关 Go 测试及 Windows 交叉编译。Windows Shell 原生结果留待下一批托管打包。

## 实际验收

- Smoke 和 release 的 Windows 脚本载入同一 YAML 定义。运行器脚本和探针的 PowerShell 语法通过，会话标记测试通过。
- 使用相同前端树复用已通过 .44 验证的前端生产资源，Go vet、全量 Go 竞态及 Windows x64 GUI 生产交叉编译通过。
- Windows 托管 Shell 原生运行待下一次合并打包。
