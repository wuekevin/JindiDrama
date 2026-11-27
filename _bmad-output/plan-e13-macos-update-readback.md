---
title: 'E13 macOS 兼容卷更新安装包回读校验'
type: bug
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'cf6e52292b37e11dd645149c3b0d5c5a728974ba'
route: oneshot
route_source: pinned
risk: high
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 3
context: [_bmad-output/plan-e13-macos-update-identity.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

macOS 兼容卷的安装包保存成功前，从实际落盘文件重新校验版本资产的大小和 SHA-256。校验失败时不给出可安装路径。发布前发现损坏时不发布。发布后出现错误时保留目标目录项并提示不要使用该文件。目录移动、目标冲突、取消及旧版 Wails 绑定的正常行为保持。

</frozen-after-approval>

## Implementation Notes

兼容卷用原打开的文件和固定目标目录核对身份，在发布前和发布后回读。发布结果确定前保持下载状态。APFS 继续使用匿名暂存与句柄克隆。

## Verification

- 真实 HFS+ 复现同一暂存文件的同长度修改，覆盖发布前、提交中、发布后身份变化、目录移动和取消。
- HFS+ 用现有 .60 ZIP 和清单做回读，核对完整字节和隔离标记。Go 全量、vet、App 与 update 竞态、Windows amd64 App 交叉编译。

## Results

- 原版在实际 HFS+ 卷上对等长内容变更返回已保存。新路径在发布前后使用版本资产提供的 SHA-256 和大小。发布后拒绝回读失败或目标身份变化的文件，并向设置页传递取消旧成功状态的结果。
- 实际 HFS+ 覆盖发布前、提交期间、发布后及回读期间的变化。测试还覆盖回读期间创建或释放可写共享映射、取消、目录移动及保留无关文件。回读实际 .60 归档的 42,934,650 字节、清单 SHA-256 与隔离标记均一致。
- 通过 Go 全量、vet、App 与 update 竞态、Windows amd64 和 Intel Mac 测试交叉编译。通过生产构建、前端类型检查、125 项 Vitest 与 Chromium、WebKit 设置页回归。

## Review Triage Log

- 已修复。HFS+ 秒级的时间戳无法提供回读期间一致性。添加 vnode 写入监听，并延伸到文件和目录身份检查完成。HFS+ 用写入代数覆盖可写共享映射，最后一次代数查询安排在最终事件轮询之后。
- 已修复。补充第一次回读后及独占发布前的改写回归。出现后续问题时，保存结果明确标记为无法确认，目录项可能需要用户检查。
- 已修复。设置页收到未验证落盘结果时移除前次保留的安装路径，正常取消仍保留前次成功状态。旧桥接保留原有错误处理。
- 系统边界。最终校验完成后，同账户可按自身文件权限继续修改文件。兼容卷仍通过按名调用执行发布，这个边界已记录。
