---
title: 'E13 Windows 安装程序拒绝被占用的升级目标'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '69d31aa11e13a32db37c85ec73c4bf1b9c5ea7f3'
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

Windows 就地升级遇到旧主程序文件被占用时返回明确失败，保留可重试的旧安装及开机启动登记。释放占用后可正常重试。静默安装也应返回失败退出码。

</frozen-after-approval>

## Implementation Notes

NSIS 在应用文件写入成功后才写安装登记。把锁文件拒绝和释放后的成功升级接入现有 Windows 安装冒烟。

## Plan Change Log

## Review Triage Log

- 低 · 初始安装与升级输入相同，可能无法发现重试未实际写入。在托管夹具中给原装程序加不同的字节标记，并对照受阻和释放后的 SHA-256。

## Verification

- 本机编译和检查 NSIS 安装器、验证内含程序字节、解析工作流 PowerShell。Windows 原生文件锁场景由下一批打包执行。

## Auto Run Result

- 本机编译 NSIS 通过，主程序和 WebView2 载荷与原件字节相同。PowerShell 解析与 smoke、release 任务共享检查通过。
- Windows 原生锁文件和解锁重试尚未执行。本轮未触发托管打包。
