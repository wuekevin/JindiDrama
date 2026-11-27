---
title: 'E13 Apple Silicon 原生验证 .61 更新保存'
type: chore
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'c8add71c8e4c23d89caf692102b9d33a11b75184'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 2
context: [_bmad-output/plan-e13-local-candidate61-packages.md, _bmad-output/plan-e13-macos-update-readback.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

在真实 macOS 桌面验证当前更新保存逻辑，包括 APFS 和 HFS+、发布后校验失败与设置页反馈，最后启动保存的实际 .61 归档。核对成功落盘摘要和隔离标记，验证原有 SQLite 资料保留。

</frozen-after-approval>

## Implementation Notes

隔离测试构建使用与 .61 一致的应用源码和前端。测试覆盖层仅把 GitHub 元数据和资产请求引向本地夹具，指定原生保存对话框起始位置，并注入一轮同长度的落盘故障。应用版本设为 .60 以发现 .61。升级时直接启动 .61 归档内的真实主程序。

## Verification

- 归档 SHA-256 与生产前端 69 项摘要一致。原生设置和系统保存对话框在 APFS 和 HFS+ 保存 .61 成品，检查真实 ZIP、隔离标记、成功路径和目标目录。
- 以有成功记录的同一原生会话重试并注入发布后错误，验证明确的 SHA-256 警告和清除可安装路径。重新下载成功，装入隔离路径。
- 应用正常退出，核对精确断点、收藏顺序、设置、搜索和 SQLite 完整性。

## Results

- 原 .61 macOS 清单六项前后独立复算通过。隔离应用内嵌前端匹配 69 项。首次直接链接缺少 UniformTypeIdentifiers，补入框架参数后正常构建、临时签名并运行。
- APFS 取消不留文件。APFS 与 HFS+ 的实际系统保存成功，完整 ZIP 字节、SHA-256 与隔离属性一致。
- HFS+ 原生界面在等长故障后给出了 SHA-256 错误，前次成功路径和复制按钮消失。修复测试目标后同一会话重试成功。
- 从重试下载解包的原始 .61 程序正常启动和退出。v10 的 SQLite、收藏、历史和精确断点、主题、启动页、预发布渠道及搜索记录均通过。HFS+ 仅为隔离内存卷，验收后已卸载。证据保存于主工作区。


## Review Triage Log

- 已修复。原 .61 在即时重试成功时仍显示前一轮未验证警告。仅撤下同一个更新警告，取消或尚未成功的操作保留提示。旧生产前端在新增浏览器断言失败，重新构建后双浏览器及原生 HFS+ 复跑通过。
- 已补证据。在复跑脚本中直接逐字节比较两个目的卷的保存副本，并记录每次比对。最后启动 .61 时初次因宿主空间不足无法写入截图文本，改在内存卷记录后复验通过。
