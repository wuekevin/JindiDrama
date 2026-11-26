---
title: 'E12 澄清短剧来源的接入与验证状态'
type: chore
ticket: ''
created: '2026-11-26'
status: built
baseline_revision: '7738aad35793d2ba3a845392da062a52bda89b56'
route: oneshot
route_source: pinned
risk: low
review: none
review_source: pinned
lenses_ran: []
review_loop_iteration: 0
followup_review_recommended: false
context: []
warnings: []
deferred: []
---

<intent-contract>

## Intent

交接说明仍把已支持的直接片源描述为只有演示或本地片库，容易误以为需要部署参考应用。将两个参考项目的接口评估、苹果 CMS 设置和红果官方接口状态连起来，并据实更新正在复验的原生播放记录。

</intent-contract>

## Implementation Notes

文档变更集中在安装说明、项目索引和 STATUS，使用单步路线。隔离源树保留精简发行说明，开发树保留完整接口核查。

- 从测试应用的 WebKit 缓存确认原来沿用单集循环。新应用标识和 SQLite 组合隔离后，无诊断探针通过四轮。

## Plan Change Log

## Review Triage Log

## Verification

- 核对 Go 片源初始化和两个暂未开启的红果 Provider，检查分发树内运行时和打包引用。
- 检查 README、AGENTS 和 STATUS 的链接及改动范围。复验记录使用原生运行输出。

## Auto Run Result

- README 说明直接片源和运行依赖，AGENTS 更新 Provider 索引，STATUS 记录 WebKit 缓存和四轮原生复验。
- 评审配置为 none，未调度评审子代理。
- `go test ./internal/provider/... ./internal/app` 通过。发行源码和工作流中没有参考应用服务配置。Apple Silicon 使用仅替代远程 HTTP 的合成夹具通过连续播放两轮、单集播放一轮、单集循环一轮，正常退出后均通过 SQLite 完整性检查。
- 真实授权端点、红果官方接口、Windows 和 Intel Mac 实机验收仍待进行。本地文档提交没有触发 GitHub 打包。
