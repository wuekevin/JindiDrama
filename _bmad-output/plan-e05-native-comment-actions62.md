---
title: 'E05 .62 归档原生评论输入与持久化验收'
type: chore
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '8f2b6ad537a7099244ca3aaa981d66b61461890d'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: [_bmad-output/plan-e05-comment-actions.md, _bmad-output/plan-e13-local-candidate62-packages.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

使用最终 .62 Apple Silicon 测试归档验证真实 WKWebView 的评论修复。通过系统键盘打开本人评论菜单，通过鼠标删除，验证焦点和 SQLite 保存。正常退出并重启，核实所删评论未恢复，另一条本人评论和演示评论保留。

</frozen-after-approval>

## Implementation Notes

独立数据目录使用演示短剧和全新评论。通过 macOS 辅助功能检查控件和焦点，实际发送键盘与指针事件，验收原归档，无业务代码覆盖层。

## Verification

- 运行前后校验 ZIP 和解出的主程序字节。
- 实测打开评论、键盘菜单、Esc、混合输入删除及重启后的页面和 SQLite。

## Results

- 解压前的 ZIP SHA-256 与 .62 归档清单一致，实际进程返回 .62 和 darwin/arm64。ZIP 与实际主程序前后 SHA-256 一致，主程序通过临时签名验证。
- 从第 32 集打开演示评论，确认占位控件和非本人删除菜单消失。实际键盘 Space 打开菜单，Esc 返回触发按钮，再次键盘打开后由系统指针点击删除，焦点返回评论输入框。
- 两条本人和 12 条演示评论已写入隔离 SQLite，删除后仅对应条目消失，其余快照内容逐项一致。两次正常退出之间重启原包，页面和 SQLite 保留相同结果，迁移记录 1–10 与完整性检查通过。
- 修正了验收辅助脚本的输入节奏、系统鼠标事件发送与只读数据库检查。验收使用原包，无业务代码改动。


## Review Triage Log

- low，已补齐。保留原归档解包应用的严格验签输出，确认该包使用临时签名。
- low，已补齐。按 Esc 后比较实际 AX 焦点元素与原菜单触发元素，区分两个同名按钮。
- low，已补齐。留存隔离数据目录及删除前后快照，独立重读重启后的 SQLite。
