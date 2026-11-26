---
title: 'E12 苹果 CMS 播放器共享倍速'
type: bugfix
ticket: ''
created: '2026-11-26'
status: built
baseline_revision: 'c1896c13e3464e4ade151f01e4bed83f2612af5d'
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

苹果 CMS 播放器读取共享倍速，但原生变更在换集、切到备用源或重启时丢失。复用播放器设置和媒体生命周期，使当前 CMS 媒体支持的 0.5x 至 2x 倍速保存至已有播放器设置，并在同一媒体补载或后续播放中恢复。原生控件报告的范围外倍速不得覆盖有效设置。

</intent-contract>

## Implementation Notes

沿用共享音量的媒体绑定和释放时机，小范围增补速度读写，使用单步路线。当前媒体沿用共享音量的追踪，在浏览器初始化和退出时都使用已有播放器 Pinia/SQLite 通路。

## Plan Change Log

## Review Triage Log

### 2026-11-26 — Review pass

- verdicts: 1 finding — high 0, medium 1, low 0, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` hls.js 原地恢复会先调用媒体 load，刚设的倍速可能在 ratechange 送达前被重置。Chromium 用 MediaSource sourceclose 复现 1.5 倍变回 1 倍。通过 hls.js 的恢复方法在重新加载之前读取并更新倍速，专项确认恢复继续解码并保留 1.5 倍。

## Verification

- 生产构建、类型检查、全量 Vitest 与目录播放 Playwright。覆盖故障和倍速变化同一事件循环、超出设置范围、手动选集与自动下一集。
- Apple Silicon 测试应用使用原生倍速菜单，验证选集和隔离 WebKit 重启后的 SQLite 持久化。
- 实测 43 项 Playwright、122 项 Vitest 通过。原生 More 菜单先选 1.5 倍，再检查下一集与重启后的选中标记，SQLite 值均为 1.5。

## Auto Run Result

- `CatalogPlaybackView.vue` 将有效 CMS 媒体的倍速写入共享设置，并在 HLS 恢复、切换备用源、切集和退出前采样。先将共享倍速用作当前值与默认值，后续加载可恢复。其他授权目录仍沿用既有倍速读取方式。
- `15-provider-playback.spec.ts` 验证 DOM 错误、实际 hls.js 恢复、范围外倍速、手动及自动切集。`docs/STATUS.md` 记录实测。
- 快审补修 1 个 medium，保留 HLS 重新加载前的最后选择，无延期项。验证后的风险仍是授权公网片源和 Windows/Intel Mac 原生环境待联调。
- 最终版本的类型检查、生产构建、122 项 Vitest、43 项 Playwright、Go vet、Go 全量和 Windows amd64 交叉构建通过。Apple Silicon 原生 More 菜单由 1 倍改为 1.5 倍，切集和保存后重开均为 1.5，SQLite 完整性为 ok。最终重开使用独立 WebKit 标识。
