---
title: 'E11 媒体保存兼容外部卷'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '02d2891897205e99a7a9ba7f2cc97a3ae37f25fe'
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

在无法原子发布暂存文件的卷上，也能保存已授权的本地片库视频和 PNG 截图。复用演示视频的独占创建和目录身份检查，保留同名文件和保存过程中被替换的目录。保持授权判断、源文件完整性和数据库备份与安装包的发布约束。

</frozen-after-approval>

## Implementation Notes

复用媒体保存兼容路径。重试从同一已打开的源视频文件开头读取，并复核属性。真实 ExFAT 验收使用一次性镜像和合成素材。

## Plan Change Log

## Review Triage Log

- low，已处理。真实卷验收增加演示视频的保存入口和同名拒绝检查。三类入口均使用合成素材及测试路径。
- low，已处理。补齐 STATUS.md 的实际日志、运行限制及首轮重试原因。

## Verification

- 给定可写 ExFAT 卷，使用原有保存入口导出允许复制的本地视频、演示视频及 PNG 截图。核对文件内容、冲突拒绝及无暂存残留。
- 给定模拟原子发布不支持且发生第二次复制，视频必须从文件开头完整读取。覆盖同名文件、目录替换与中断行为。
- Go 全量、vet、App 竞态与 Windows amd64 交叉编译。

- 挂载真实 ExFAT 复现并验证三类媒体保存、同名文件保持、权限和暂存清理。Go 全量、vet、App 竞态及 Windows amd64 交叉编译通过。
