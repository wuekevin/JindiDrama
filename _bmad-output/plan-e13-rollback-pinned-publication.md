---
title: 'E13 固定回滚发布的源和目标目录'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '9db0e0e79e117b2140ebd2345879e4e54401f8e6'
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

回滚数据库在私有工作目录完成降级和完整性检查。打开用户选择的目标目录后，后续复制和独占发布通过这个目录句柄完成。回滚期间目标路径改变或出现同名 SQLite 文件时拒绝发布，保留替换目录和原始源数据库。

</frozen-after-approval>

## Implementation Notes

用户缓存目录承载 SQLite 工作副本。已检查的成品复制到固定目标目录下的私有暂存区，再通过两个目录句柄原子创建硬链接。复用现有权限保护与 SQLite 快照校验。

## Plan Change Log

## Review Triage Log

- medium，已修复。使用项目依赖已有的 Windows NtCreateFile 打开相对于目录句柄的文件，SQLite 与 App 均通过 Windows amd64 交叉编译。
- medium，已处理。硬链接使用原目录句柄，创建前后均检查目标路径和辅助文件。写入瞬间发生的变化会报告失败和副本位置，保留已发布的原位置文件及外部辅助文件。单个独占硬链接无法同时锁定其他文件名。
- medium，已隔离输出目录。SQLite 仅在用户缓存的私有工作区降级，目标目录的替换不会重定向 SQLite。工作区本身按受保护的用户目录使用，其他同用户进程改写该工作区的竞态仍不在本次覆盖范围。
- low，已记录。上一轮的空目录清理仍使用身份检查后执行目录专用删除，验证替代目录中的文件不会被清理。

## Verification

- 给定回滚期间移动所选目标目录，降级仍在私有区执行，替换目录和已在文件保持不变，且不会返回成功。
- 给定后出现同名文件或 SQLite 辅助文件，命令报错并保留它们。给定原生目录句柄及其原路径的替换目录，硬链接仅使用原句柄。
- 旧版兼容副本、ACL、活动 WAL、源数据保留、Go 全量、vet、竞态及 Windows amd64 SQLite 交叉编译。

- macOS 回归、实际回滚 CLI、Go 全量、vet、App 和 SQLite 竞态及 Windows amd64 两套测试交叉编译通过。
