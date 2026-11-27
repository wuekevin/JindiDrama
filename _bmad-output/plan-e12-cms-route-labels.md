---
title: 'E12 苹果 CMS 播放线路名称'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'c0e7419ec50d129f7fe93d8360cfed8a6b48ec98'
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

经配置的苹果 CMS 提供可读播放组名称时，播放器应显示对应名称。缺少名称、名称无效或分组数量不匹配时，继续使用线路编号并保持播放和切源能力。

</frozen-after-approval>

## Implementation Notes

源名称依据 MacCMS V10 的 vod_play_from 和 vod_play_url 按原始位置匹配。兼容原始分隔符与官方按来源筛选后的逗号形式。名称只随已验证的播放源传递。URL 继续通过现有 Go 租约代理处理。

## Verification

对 Go Provider、真实 Go 播放票据和 Chromium、WebKit 切源路径做回归。验证缺失、错配、重复地址和无效标签。在原有 1448×1086 和 220% 布局对照播放源菜单。

## Results

- Go Provider 和 Go 桌面环回票据保留对应线路名，地址是否可播放仍由现有媒体与租约检查决定。重复地址选择首个分组。无名、错配和不支持的名称回退为编号。
- 全量 Go、vet、前端生产构建、Vitest 125 项、Chromium 播放 70 项和 WebKit 切源 5 项通过。Chromium 一条既有回退用例首轮超时，独立三次与全套复验均通过。对照默认和 220% 播放布局。
- Apple Silicon 合成目录实测三个名称通过真实 Go 票据进到 WKWebView，仍可暂停切线及从 404 回退继续播放。正常退出并通过隔离 SQLite 检查。

## Review Triage Log

- 中等。仅显示多源菜单会遗漏单条命名线路，已加静态名称和浏览器回归。
- 中等。WebKit 220% 下 32 字长名称令下拉框溢出，已限制控件宽度，测试核对完整控件和页面宽度，菜单仍提供完整文本。
- 低。可读括号、加号和组合音标已纳入验证。超过 32 组时核对全量数目和位置，仅截取可保留的组。补充两项 Go 回归。
- 低。修正所引用 MacCMS 源文件的实际行号。
