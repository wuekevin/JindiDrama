---
title: 'macOS 开机启动识别系统禁用状态'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: c7cb0307216e6b8d09e1086748a17631f59f2be5
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

有效的 LaunchAgent 文件仍可能被当前 macOS 用户会话禁用。设置应反映 launchd 已明确禁用的状态，尝试重新开启时应提示用户检查系统设置，并保留原登记与系统禁用设置。

</frozen-after-approval>

## Implementation Notes

复用现有 LaunchAgent 校验。仅在需要确认已存在登记或启用时查询当前 GUI 会话，识别金帝自己的标签。系统查询失败走现有读取或写入失败状态。

## Verification

- macOS 夹具覆盖已允许、已禁用、无登记和系统状态读取失败。
- 只读核对本机 launchctl 响应格式。复跑已有真实 plist 与应用模块测试。


## 实际验收

- Apple Silicon 本机只读运行当前 GUI 域的查询，生产解析逻辑正常返回。本机金帝标签没有显式禁用覆盖。测试夹具核对系统禁用、未知响应、兄弟标签及保留已有登记。
- Go vet、全量 Go、开机启动竞态以及 Windows amd64 开机启动测试程序交叉编译通过。实际注销重登与系统后台项审批仍待验收。
