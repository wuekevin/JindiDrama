---
title: '生成安装包 SHA-256 校验清单'
type: chore
ticket: ''
created: '2026-11-25'
status: built
baseline_revision: 'd5e3579cf214afe69feb884dc7b79f055965ed0b'
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

**问题**　E13-S01 要求打包工作流提供哈希。现有 GitHub Actions artifact 包含 Windows 安装包和 macOS ZIP，下载后缺少可单独验证文件的校验清单。

**方案**　沿用现有两端共用的 Node.js 环境，在产物完成签名、归档及平台检查后生成 SHA256SUMS.txt，并与对应平台的产物一同上传。使用标准的双空格格式和相对文件名。

- 给定包含实际待上传 EXE 或 ZIP 的打包目录，当生成清单时，每个 EXE 和 ZIP 都有且只有一条与最终字节相符的 SHA-256 记录。
- 给定打包目录中没有 EXE 或 ZIP，当生成清单时，命令失败并令上传步骤跳过。

</frozen-after-approval>

## Implementation Notes

单次实施路线。变更集中在打包脚本和工作流，沿用现有构建及上传边界。

清单按文件名排序，流式计算哈希。GitHub Actions 两个平台共用脚本。

## Plan Change Log

## Review Triage Log

快速评审确认两处上传匹配问题和一处验证记录缺口，均已修正。深入评审未启用。

- 中，已修复。Windows 的上传通配符忽略扩展名大小写，脚本现按平台匹配。
- 中，已修复。GitHub Actions 默认排除隐藏文件，脚本现同步排除点号开头的文件。
- 低，已修复。实际测试证据已写入 `docs/STATUS.md`。
- 未确认。评审草稿提到任意 POSIX 文件名的换行和反斜杠。现有 Wails 和 ditto 产物名称固定，包括从当前工作流传入的路径，保留当前清单格式。

## Verification

- 在临时目录准备 EXE、ZIP 与其他文件，运行生成命令，再用 `shasum -a 256 -c SHA256SUMS.txt` 交叉验证。覆盖空目录、带空格的文件名和文件篡改检测。
- 检查 GitHub Actions YAML 解析、步骤顺序和上传清单路径。

本机验证全部通过，包含 GitHub Actions YAML 解析与步骤断言、校验和夹具、Go vet 和单测、前端类型检查与构建、Vitest 48 项、Playwright 136 项。GitHub Actions 打包尚未在双平台运行。
