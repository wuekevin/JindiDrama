---
title: 'E12 苹果 CMS 播放器共享音量'
type: bugfix
ticket: ''
created: '2026-11-26'
status: built
baseline_revision: '686a242c0d8214831d17ad29f5cb0feefe72384e'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 0
followup_review_recommended: false
context: []
warnings: []
deferred: []
---

<intent-contract>

## Intent

苹果 CMS 播放页读取共享音量，但原生控件的音量变化不会写回，手动选集、自动下一集或切换备用源后会恢复旧值。沿用同一个播放器音量设置和已有桌面持久化，使当前有效媒体的调节持续生效。自动下一集的静音仍按现有行为处理，静音不会把共享音量当作零。

</intent-contract>

## Implementation Notes

缺口集中在一个 Vue 播放器，把原生事件接到已有 Pinia 设置即可，适合单步修复。播放器新媒体元素沿用已归一化的共享音量。仅当前路由、当前媒体与有效票据可回写音量，复用设置的 Pinia/SQLite 入口。

## Plan Change Log

## Review Triage Log

### 2026-11-26 — Review pass

- verdicts: 2 findings — high 0, medium 1, low 1, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` 换源时排队中的音量事件可能晚于解码器错误。测试在同一个 JS 任务内调音量并触发故障，旧实现复现 70% 回退。修复在撤换播放器前采样当前有效媒体。
  - `[low]` `[patch]` 通用授权播放共用此视图，初版修改也会将该路径的音量写入共享设置。把回写绑定至已识别的 CMS 播放媒体，通用授权播放仍能读取共享音量。

## Verification

- 在现有真实 Chromium 媒体夹具选择音量，验证备用源、手动切集、静音和连续播放状态。
- 在 Apple Silicon WKWebView 通过原生控件修改音量，验证选集、退出后的 SQLite 与独立 WebKit 重启。
- 运行类型检查、构建、Vitest、全量 Playwright、Go vet 与 Go 全量测试。现有环境实测 122 项 Vitest、363 项 Playwright，通过。最终原生音量约 20%，选集和重启后仍为 20%，SQLite 检查通过。

## Auto Run Result

- CMS 播放器通过同一个 Pinia 设置保存原生音量；换集、备用源和自动下一集读取共享音量。测试在解码故障的同一个任务中修改音量，修复后仍能将 23% 应用到备用源。
- `CatalogPlaybackView.vue` 跟踪当前 CMS 媒体并在有效期内同步音量。`15-provider-playback.spec.ts` 覆盖备用源时序、手动切集与自动切集。`docs/STATUS.md` 保留本次验证。
- 快审处理 1 个 medium、1 个 low。现有媒体在释放前采样最新值，回写只对 CMS 生效。无延期或驳回项，无需追加评审。
- 最终类型检查、生产构建、122 项 Vitest、363 项 Chromium、Go vet、Go 全量、Windows amd64 生产交叉构建、12 项 Node 打包校验及 11 项 macOS 冒烟和签名脚本通过。
- Apple Silicon 原生播放器从 SQLite 读入 18%，调整后在切集和更换 WebKit 标识重启时读到约 20%。SQLite 保存 20%，完整性为 ok。网络使用合成 CMS/MP4 夹具，真实公网源、Windows 和 Intel Mac 原生仍待联调。
