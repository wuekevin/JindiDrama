---
title: 'E13 用 GitHub 公开接口核对发布标签'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '6bf36018d0154ed7822d5bc02f9c243c5aae03f2'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

正式交付需要把 GitHub 上的发布标签和实际打包提交核对。本地 Git 的 URL 重写可能把请求导向落后的镜像。草稿、发布后复验及公开下载校验使用 GitHub 官方公开接口检查准确的标签命名空间，轻量标签和附注标签都解析到提交。远端标签缺失、对象不合法或提交不符时停止。保留本地标签校验和其余交付检查。

</frozen-after-approval>

## Implementation Notes

GitHub Git References API 给出标签对象。附注标签使用 Git Tags API 逐层解析。公开查询使用同一超时和固定 GitHub API 地址。测试复用交付夹具，保留只向本机 bare Git 镜像重写的配置，用独立的 API 响应模拟公开端变化。

## Verification

- 用镜像一致但 GitHub 缺失或已改动的夹具复现原版误验收。新版本停止。
- 草稿、发布后和公开文件下载回归通过，覆盖轻量、附注、嵌套附注及缺少对象。
- 公开 GitHub Git 数据库只读连通测试，真实正式发布仍需实际签名成品。

## Results

- 原版误将本机 bare 镜像当作 GitHub，忽略模拟公开端的 404。修复后正式交付只使用 GitHub Git References 和 Git Tags 公开接口核对远端。镜像和 GitHub 标签不一致时，由公开端的准确标签决定。
- Node 22 发行校验 18 项通过。覆盖标签缺失、移动、错误引用命名空间、轻量、附注与嵌套附注，以及坏对象、缺失对象和循环。草稿、已发布版本与公开附件交付夹具均通过。
- 真实 GitHub 上用 Node.js v22.0.0 的附注标签及 Vite v5.0.0 的轻量标签，分别与独立提交 API 比对成功。现存项目 .52 两份 GitHub 原始归档和 13 项内部 SHA-256 复验通过。
- 草稿、已发布版本和公开附件的完整流程仍需实际签名 Release。上述实网测试覆盖公开标签接口。

## Review Triage Log

- 低，已修复。实网证据分别记录独立提交 API 和发布标签解析器得到的摘要及逐项相等断言。
- 低，已修复。补充发布后和公开下载模式下远端 404 的失败断言，核对流程不会放行附件。
- 低，已修复。使用 intent-to-add 将计划纳入隔离同步补丁，校验开发分支中的 BMad 链接。
