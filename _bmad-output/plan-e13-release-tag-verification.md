---
title: 'E13 将发行标签与交付包绑定'
type: bugfix
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: '2e90c14eeddfd6ba038a9ac002ec1c707901386e'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

在发布前验证正式候选交付包的文件哈希、拟发布标签和构建提交。标签指向错误提交、版本不一致或交付包已经损坏时命令须失败。支持轻量标签与附注标签。

</frozen-after-approval>

## Implementation Notes

复用发布说明格式和交付包检验，校验命令只读本地仓库与交付文件。

## Plan Change Log

## Review Triage Log

## Verification

- 运行交付组装和校验器的 Node 测试。用隔离的真实 Git 仓库覆盖标签解析、不同提交和篡改产物。
- 从开发树和隔离发行树核对 README 中的命令和验收记录。

## Auto Run Result

- 复用现有正式交付清单和发行说明格式，将本地标签指向的提交和打包来源绑定。
- Node 22 和 Node 26 各 13 项工具回归通过，旧工具未实现新命令。轻量及附注标签可核验，错误提交、版本、缺标签、同名分支和文件篡改均被拒绝。
- .41 测试包全部 13 项 SHA-256 兼容性复验通过。正式签名和公证的交付包仍待实跑。
