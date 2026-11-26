---
title: 'E13 桌面程序内置回滚命令'
type: feature
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '0c8c02d336c1556d75effe66bbf2b1e18dfb41f0'
route: oneshot
route_source: auto
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

桌面安装用户可以在退出金帝短剧后直接使用当前程序生成旧版兼容 SQLite 副本。命令不启动窗口，不改写原数据库。参数或目标位置无效时返回失败，源码工具仍可使用。

</frozen-after-approval>

## Implementation Notes

共享已有源码工具的命令解析和返回值；桌面启动在日志、WebView 和其他桌面服务之前识别回滚选项。

## Plan Change Log

## Review Triage Log

- 低 · 补录实际验证和未运行范围至 STATUS.md。
- 低 · Windows 示例补充按用户安装、自定义目录和便携版路径。

## Verification

- 运行 Go 单测、Go vet、Windows amd64 桌面编译。Apple Silicon 真正调用当前桌面二进制，将合成 v10 配置转为 v2，使用旧版真实数据层读取，再校验原库摘要。

## Auto Run Result

- Go vet、Go 全量、回滚与 SQLite 竞态检测通过。Windows amd64 GUI 版交叉编译通过。
- Apple Silicon 原生应用成功生成独立 v2 副本，旧版 Go 数据层读回收藏、进度、搜索及设置。原库摘要及互动保留，现有目标未覆盖，应用配置未创建。
- 本地测试归档普通启动与正常退出通过。Windows 原生回滚待执行。
