---
title: 'E13 更新下载暂存清理反馈'
type: bug
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '3c49248dad27fe7a67e5422932fb605783005843'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 4
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

当更新下载的暂存目录清理失败时，桌面反馈能区分未保存和已校验保存。普通取消仍保持静默。更新下载失败时保留失败原因，清理提示不暴露内部路径。任何分支均保留目录里无关的文件。

</frozen-after-approval>

## Implementation Notes

复用平台暂存清理回调，在更新保存返回前报告清理结果。通过现有桌面 Wails 错误提示展示结果。

## Verification

- 在真实暂存目录放入独立测试文件以触发清理错误。覆盖取消、校验失败和已验证后保存三种路径，检查结果、最终安装包、暂存安装包和独立文件。
- Go 全量、vet、App 竞态、Windows amd64 App 测试交叉编译及更新桥接的设置页回归。

## Results

- 原版未报告实际暂存目录不能删除。修改后普通取消依旧静默，取消、校验失败和已保存后的清理错误分别按结果展示。校验失败或取消从未发布测试包。来源标记或已经建立的暂存区初始化失败时，反馈会携带经过清理的错误文本。
- Wails 结果可同时携带已经校验的保存路径及提示，现有两个下载绑定保留。Chromium 和 WebKit 设置页专项各 2 项通过，覆盖警告后的路径复制、旧版桥接及既有取消与失败提示。类型检查、生产构建和 125 项 Vitest 通过。
- Go 全量、vet、App 和 update 竞态、Windows amd64 交叉编译通过。专项覆盖同时取消的下载校验、文件写入及连接失败，以及 macOS 重命名完成后的独立同名文件保留。实际的 Windows 桌面测试待累计打包。
- 磁盘空间不足导致全量 Go 链接失败，删除此前已归档测试包的临时副本后重新执行通过。

## Review Triage Log

- 已修复。Wails 错误返回会丢弃同时返回的路径。新增兼容绑定返回路径和警告，浏览器实测仍能复制路径。
- 已修复。发布后目录移动且清理失败时，提示查找原目录。
- 已修复。macOS 和 Windows 在文件创建或来源标记失败后的清理问题会进入统一提示。macOS 无法打开已建立的暂存目录时也会报告清理未确认。
- 已修复。实际返回的读取、连接、校验及本地写入错误在同时取消时得以保留。没有已返回错误的普通取消继续静默并停止保存。
- 已修复。macOS 独占重命名成功后不再触碰暂存安装包原名称。
- 延后。macOS 发布前和硬链接回退仍需专门核查文件身份，记录于 deferred-work.md。
- 已修复。验收结果及证据已写入 STATUS.md。
