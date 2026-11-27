---
title: 'E04 苹果 CMS 手动切换播放源'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'eff75f240fa60a2ce9ead9f9ee79e7ac571f47a9'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

苹果 CMS 有多条可用播放源时可手动选择。仅显示上游明确提供的画质标识，其他线路按顺序标示。切换保留播放或暂停意图和当前断点。手动选中的线路故障时仍然可以回到本轮其他可用线路，避免连续尝试已经失败的线路。

</frozen-after-approval>

## Implementation Notes

复用已验证票据内的线路、媒体进度恢复和备用源路径。选择感知播放票据及媒体生命周期，无需改变上游接口。

## Verification

- 用双线路真实样片复现原版缺少控件，验证暂停定位、手动切换和选中的最后一条线路失效后的回退。
- 核对控制在默认和 220% 下可用，跑 CMS、前端单测和构建。

## Review

- 修正暂停切源后再次拖动进度时回退使用旧断点的问题。定位完成时更新本次播放的内存断点，观看历史仍遵循实际播放规则。
- 修正切源缓冲时主动暂停后备用线路自动启动的问题。暂停清除待执行的自动播放，正常的暂停中断不显示浏览器阻止自动播放的提示。
- 补充 HTTP 404 媒体响应的回退用例和默认、220% 的界面对照。

## Results

- Node 22 生产构建和 Vitest 125 项通过。Chromium Provider 播放全套 69 项及 WebKit 切源专项 4 项通过。
- 默认窗口和 220% 时播放源控件可见，键盘焦点可见。画中画随控件行换行，本机评论和本机弹幕入口完整。
- 使用本地样片和模拟 CMS 播放票据。改动纳入下一次累计桌面打包验收。
