---
title: 'Apple Silicon 原生最小化与续看验收'
type: chore
created: '2026-11-27'
status: built
baseline_revision: 14ce2ff7733f6bf8601685843b8b5fdede69e925
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

在当前 Apple Silicon 桌面候选中，验证真实 WKWebView 经顶栏最小化时保存演示和本地片库的播放进度，并验证恢复窗口、正常退出和 SQLite 完整性。

</frozen-after-approval>

## Implementation Notes

使用最后一轮通过测试的内嵌前端构建和当前隔离发行源码。仅在本机以独立开发标识运行。媒体使用仓库自制样片，配置与数据库均隔离。

## Verification

- 核验生产构建、arm64、最低系统版本和本机临时签名。
- 通过系统辅助功能操作真实播放器和最小化按钮。在定期记录前确认 SQLite 获得新进度，再验证原生窗口恢复和退出后 SQLite。


## 实际验收

- 源码对应发行候选 `14ce2ff7733f6bf8601685843b8b5fdede69e925`，复用该候选通过 164 项相关 Playwright 的内嵌资源。macOS 26.6.2、arm64，最低系统版本 13.0，ad-hoc 签名校验通过。首次编译因磁盘空间不足中断，清理 .46 已归档的临时重复副本后重编译成功。
- 演示视频在顶栏最小化时写入 98 ms，片库视频关闭到任务栏时从 5800 ms 更新到 5993 ms，均早于各自定期记录间隔。恢复窗口后仍在播放。退出均正常，SQLite 完整性均为 ok。
- 使用退出后的同一隔离数据库重开应用。演示恢复暂停位置 1298 ms，界面显示 1 秒。片库以暂停状态打开第 1 集，点击播放从 7 秒继续，退出前保存的位置是 7161 ms。
