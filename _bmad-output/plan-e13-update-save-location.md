---
title: '设置显示已校验安装包的保存位置'
type: feature
created: '2026-11-27'
status: built
baseline_revision: ab9bd966610937ab754d3b4ac9305b5b02dbbd11
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

用户保存经校验的更新包后，可在设置页看到并复制准确的本地路径。取消或保存失败时不产生保存回执。重新检查或切换更新渠道时清除旧候选的路径。

</frozen-after-approval>

## Implementation Notes

新桌面桥接返回已成功发布的文件路径。旧布尔桥接继续支持。仅在当前应用会话内显示路径。用户从文件管理器手动安装。

## Verification

- 浏览器用桌面桥夹具验证取消、保存、复制、重新检查及紧凑布局。
- 前端类型检查、构建、单元和设置页回归。Go 下载、发布和绑定测试。


## 实际验收

- Chromium 桌面桥夹具验证取消下载无新回执，保存成功后路径自动滚动可见。含中文和空格的长路径完整复制。重新检查与切换渠道后清除旧路径。
- 对照设置页设计稿复验 1448×1086、1280×860 与 220% 缩放。首轮发现短暂成功提示盖住紧凑布局的回执区域，改为设置页内状态后复验。220% 下长路径换行，复制可从键盘触发。
- 前端类型检查、生产构建、122 项 Vitest、44 项设置 Playwright 和新回执独立复跑通过。Go App 与更新模块测试及 vet 通过，Windows x64 生产交叉编译通过。真实项目 Release 的下载和系统剪贴板联调待验收。
