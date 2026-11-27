---
title: 'E05 评论只显示可执行的交互入口'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '51aa35b8bebc02160bd3e791f9ea30a40a49cded'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

演示与本机评论使用同一条能力边界。评论面板只提供实际可执行的操作，个人评论才能显示删除菜单。键盘能展开、收起并执行删除，删除后可继续编辑。演示评论、表情、点赞和回复继续使用本机存储。

</frozen-after-approval>

## Implementation Notes

删除四个只显示不可用提示的入口。复用已有的本机评论删除和保存逻辑，按所有权渲染菜单。评论面板沿用现有的关闭键和焦点行为。

## Verification

- 先用原始生产构建触发回归失败，留存 1448×1086 原始截图。
- 类型检查、Vitest 和生产构建。Chromium 评论、WebKit 键盘回归，以及 Provider 的本机评论相关用例。
- 核对修改后的 1448×1086 和 220% 界面，验证键盘操作和删除后持久化。

## Results

- 原应用的失败回归确认演示页仍提供不可用的图片入口。删除模拟按钮后，评论、表情、点赞和回复回归均通过。
- 先在 1448×1086 核对原样和改后截图，再在 1280×860、220% 缩放下实测键盘到达删除、Esc 恢复焦点以及删除后的编辑焦点。首轮尝试在已经展开菜单时改变视口，浏览器未自动滚动到已聚焦项。最终用例以目标缩放实际重新打开菜单。
- 原有保存冲突的跨窗口用例验证删除入口禁用。CMS 夹具验证按分集保存删除、旧记录只读、切集及重启不混用。
- TypeScript 检查、生产构建、125 项 Vitest、40 项 Chromium 和 4 项 WebKit 通过。

## Review Triage Log

- medium，已修复。保存冲突会禁用删除菜单触发器，并收起已打开的菜单。跨窗口冲突回归覆盖此行为。
- medium，已修复。打开其中一个弹出区会收起另一个。测试先打开表情，再进入评论的菜单，验证 Esc 返回该评论。
- medium，已修复。WebKit 复现键盘打开后点击删除会先失焦。删除项保留按下时的焦点，混用回归通过。
- low，已补齐。验收和截图链接已加入 `docs/STATUS.md`。
