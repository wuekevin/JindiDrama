---
title: 'E11 Windows 交互托盘验收'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '2faf14ccea08c28d86b95c0e8e211a9c7d4ade4c'
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

Windows 成品冒烟覆盖了回退最小化和模拟托盘通知。上线前仍需证明确实能通过 Explorer 通知区的菜单恢复原窗口，以及 Explorer 重启后可以再次使用托盘菜单退出。

给现有隔离冒烟脚本增加交互场景。脚本核验真实菜单恢复、Explorer 通知区进程变更、正常退出和 SQLite。交互操作安排在可重启 Explorer 的 Windows 测试桌面。

</frozen-after-approval>

## Implementation Notes

复用 Windows 探针和 PowerShell 隔离配置。探针读取通知区窗口的进程编号。脚本等待测试者点击真实菜单或重启 Explorer，自动核对测试应用状态。

通过 `GetGUIThreadInfo` 观察托盘窗口所属线程的弹出菜单及菜单所有者。脚本先确认菜单打开，再提示选择恢复或退出。Explorer 重启前后比较通知区进程，退出后复用 SQLite、会话标记及回滚检查。[Win32 接口契约](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-guithreadinfo)

## Plan Change Log

## Review Triage Log

- 仅检查恢复状态无法确认弹出了托盘菜单。已将菜单所有者和弹出状态纳入每一次交互的前置检查，菜单选项由测试者实际操作。
- 已将本地验证证据和未执行的 Windows 实机验收范围记入状态文档。

## Verification

- Windows amd64 交叉编译 Win32 探针，运行 Go 测试和 vet。
- 解析 PowerShell 脚本，执行可在 macOS 运行的 PowerShell 测试和参数检查。
- 普通 Windows 交互会话从现有 .51 成品运行，仅在真实 Shell 操作通过后记录原生验收。
- macOS 上 Go 测试、PowerShell 解析和夹具通过。Windows amd64 交叉 vet 与探针构建通过。真实通知区仍待 Windows 交互桌面支持。
