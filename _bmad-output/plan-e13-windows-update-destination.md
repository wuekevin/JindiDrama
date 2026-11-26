---
title: 'Windows 更新包保存目录身份校验'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 271ce965979a2857268e6180b11a797cd85cf3f3
route: oneshot
route_source: pinned
risk: medium
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

Windows 校验下载更新时，将完整安装包保存到用户原先选择的目录。目录被移动、替换或改向时给出与 macOS 一致的结果，保留同名文件和下载来源标记。

</frozen-after-approval>

## Implementation Notes

使用原目录与私有暂存目录的打开句柄执行创建、重命名和清理。保存前后复用目录身份检查，区分尚未写入和已在原目录写入的情况。

## Verification

- Windows 回归覆盖路径改向、正常发布与来源标记。
- 本机执行 macOS 目录身份和下载回归，Go 全量及专项竞态，交叉编译 Windows 生产应用和 Windows 原生测试程序。

## 实际验收

- macOS 原生 Go 回归覆盖目录移动和替换、发布时变化、来源标记、取消及成功保存。全量 Go 与专项竞态通过。
- Windows x64 静态检查、测试二进制以及生产 GUI 交叉编译通过。新 Windows 用例需要在下一批 CI 中执行。
- 初轮并行构建耗尽磁盘空间。核对已有归档后清理旧临时构建副本，按顺序重新执行并通过。
