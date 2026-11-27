---
title: 'E12 同剧切集沿用苹果 CMS 播放线路'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '855e34a066a836c57dd648b1f2e1784f373a051a'
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

同一部苹果 CMS 剧目手动切源后，手动或连续播放切集应沿用新票据中唯一匹配的名称。名称缺失或重名时保留按原顺序选源。自动故障回退不应改变用户本次观看明确选中的线路。

</frozen-after-approval>

## Implementation Notes

在当前播放页内记录剧目和唯一的线路名。每次加载仍取当前分集的新票据，并从已通过既有地址和权限校验的线路中选择。离开剧目或结束播放页即清除记录。

## Verification

以真实样片验证手动切集、连续播放、重名与缺名回退及离开后的重置。运行生产构建、Vitest、Chromium 播放套件和 WebKit 切源专项。

快审未发现产品逻辑缺陷。回归用例补上自动下一集的备用线路位置变化，以及找不到所选名称时使用首条线路，之后仍可沿用此前的手动选择。

原版 WebKit 用例复现切集后回到首条线路。修复版前端生产构建、Vitest 125 项、Chromium 播放 71 项和 WebKit 切源 6 项通过。补齐线路换位与缺名后，Chromium 单项与 WebKit 6 项均通过。Chromium 单项曾因临时磁盘空间不足无法写入截图，清理隔离构建缓存后通过。

Apple Silicon 的 Wails 隔离构建使用真实媒体样片与合成目录。手动切到第二集沿用备用线并保持暂停，自动进入第三集沿用备用线且保持静音，两个分集使用各自的新票据。原生退出和 SQLite 完整性通过。
