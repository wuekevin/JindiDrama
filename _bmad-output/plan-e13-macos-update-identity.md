---
title: 'E13 macOS 更新安装包暂存文件身份'
type: bug
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '5410feff49768a861d89b6aff564684bcec0cefb'
route: oneshot
route_source: pinned
risk: high
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 2
context: [_bmad-output/plan-e13-update-stage-cleanup.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

macOS 更新发布使用校验过的安装包。APFS 下载过程及发布直接使用保持打开的文件。兼容卷保留独占发布，并在发布前后检查文件身份。清理遇到身份不符时保留无关文件。目标冲突、取消、原目录移动和 macOS 文件隔离标记继续有效。

</frozen-after-approval>

## Code Map

- `internal/app/update_stage_darwin.go` 创建私有暂存区、设置隔离标记、独占发布及清理。
- `internal/app/update_download.go` 在大小与 SHA-256 校验后提交，负责保存结果及提示。
- `internal/app/update_stage_darwin_test.go` 已覆盖固定目录句柄及保留同名文件。

## Implementation Notes

APFS 更新暂存使用原生 `fclonefileat`，原生探针已确认可以克隆无目录项的打开文件并保留隔离扩展属性。其他支持独占发布的卷维持目录句柄及原生重命名或硬链接，补充身份检查。标准媒体、截图及备份保存沿用现有机制。

## Verification

- APFS 实际 Go 文件回归验证无目录项下载、无关文件保留、正确的 SHA-256 与隔离属性、目标文件不覆盖以及取消。
- 兼容卷身份保护专项与现有选定目录移动、媒体输出、备份及更新下载回归。
- Go 全量、vet、App 竞态、macOS 其他架构或 Windows 可用的交叉编译。

## Results

- 原版回归未通过，下载过程中有可写目录项并在清理时消失。新 APFS 回归证实传输前暂存句柄已无目录项，实际落盘文件和独立同名夹具都符合预期。
- 归档 .60 ZIP 在当前 macOS 保存路径回读 42,934,650 字节，SHA-256 与归档清单一致，隔离标记存在。真实 HFS+ 卷上正常发布并保留隔离标记，身份改变及取消路径保留无关文件。HFS+ 卷已卸载。
- 目标在发布前抢先出现时按实际发布冲突处理，原内容保留，无暂存目录残留。已有下载、目录变化和媒体保存回归通过。
- Go 全量、vet、App 竞态及 Windows amd64 交叉编译通过。本机短时空间不足导致链接和 SQLite I/O 失败，清理旧临时测试产物后复验通过。

## Review Triage Log

- 已修复。初始化补入摘除前身份校验及摘除后打开文件链接计数确认。给定同账户进程可以并发更改同一个私有目录，该路径检查与摘除仍不能合成单个原子操作，边界已记录。
- 已修复。补充同名文件在最初路径检查之后出现的独占发布回归。
- 延后。兼容卷在发布前后核对身份并将发布后不符视为保存异常。底层按名称执行的重命名与清理，以及同一文件的内容变化还需进一步核查。APFS 下载及最终克隆不使用暂存名称。
