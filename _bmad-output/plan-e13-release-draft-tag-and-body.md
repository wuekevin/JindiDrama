---
title: '核验 GitHub Release 草稿正文与远程标签'
type: bugfix
created: '2026-11-27'
status: built
baseline_revision: 5562919245c60d20016557a9ea2f9986a4a4215f
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

发布前的既有只读草稿校验同时确认实际 GitHub 标签指向打包提交，以及公开的说明正文与已校验的交付说明一致。任一差异须在发布前报错。

</frozen-after-approval>

## Implementation Notes

复用交付包的提交、仓库和说明。通过 Git 精确查询远程标签，并读取附注标签的解引用提交。正文比较容许换行编码和末尾空白差异。

## Verification

- 本机 Git 远程夹具验证附注及轻量标签，拒绝不存在或已移到其他提交的远程标签。
- Release 草稿夹具验证正文和上传资产，使用 Node 22 与系统 Node 运行。

## 实际验收

- 本机隔离 Git 仓库验证远程附注和轻量标签。远程同名分支存在时，标签缺失仍失败。远程标签移动、本地标签错误及草稿正文改动均被拒绝。
- Node 22 与 Node 26 各通过 13 项发行脚本测试。现有 .45 Windows 和 macOS 共 13 项 SHA-256 重新验证通过。
- 已只读核对 GitHub 当前无正式发行记录。实际签名交付包和线上草稿仍待验收。
