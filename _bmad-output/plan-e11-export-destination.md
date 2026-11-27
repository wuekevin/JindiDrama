---
title: 'E11 保存媒体时固定目标目录'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '3382553aff2b809b7e1532c3d56103f1b633ef84'
route: oneshot
route_source: auto
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

视频导出和截图在复制结束后按原路径发布、清理暂存文件。目标目录若在复制中被移动并原路径出现新目录，路径可能指向与用户选择不同的文件。

复用已用于更新保存的目录句柄和独占发布。覆盖演示视频、本地授权视频与截图。保留演示视频在不支持独占发布的文件系统上的直接复制路径、失败反馈及禁止覆盖行为。

</frozen-after-approval>

## Implementation Notes

测试在独立发行源码执行。发布和清理均作用于已打开的目录和实际暂存文件。媒体保留常规本地来源语义，更新安装包继续由现有平台标记来源。

保留演示视频兼容路径。独占发布不支持时，先移除暂存数据，再从已打开的原目录独占创建最终文件。目录变更发生在发布前后时，给出不同的保存位置提示。

## Plan Change Log

## Review Triage Log

- 传播暂存清理错误。清理失败时停止回退复制，已发布的结果使用独立提示。
- Windows 媒体采用目标目录继承的权限。安装包继续使用专属权限及来源标记。
- 通用 POSIX 发布使用两个已经打开的目录句柄，目录替换回归按完整相对暂存路径放置对照文件。

## Verification

- 复制中移动目录并建立同名替代目录，确认替代目录及用户已有文件的内容保持正确。
- 运行现有导出、截图及安装包下载回归，包括跳过覆盖和演示视频回退。
- Go 全量、vet、App 竞态和 Windows x64 交叉编译。
- 旧版定向测试复现目录替换后错误返回成功。修复后原生 macOS Go 专项、全量、vet 和 App 竞态均通过。Windows 与 Linux amd64 App 测试程序交叉编译通过。ExFAT 实盘与 Windows GUI 留待真实环境。
