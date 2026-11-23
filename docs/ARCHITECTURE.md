# 架构与技术决策

## 栈
Wails v2 / Go / Vue 3 + TypeScript / Vite / Pinia / Vue Router / SCSS + CSS Variables / Lucide / SQLite（WAL） / Vitest / Playwright。Media 层优先合法的播放源与系统/WebView 解码能力，必要时可引入已有合法授权的解码器。Windows 优先；macOS 按原生构建及签名流程交付。不使用 Docker。

## 依赖方向
前端 View → Pinia Store → Typed JS Bridge → Go App Service → Provider Interface → [MockProvider | AuthorizedProvider]。SQLite/Secret Store 由 Go Service 使用；界面严禁直接调用非公开 fqnovel/snssdk 端点。

## Go 目录（目标）
```
cmd/desktop/main.go
internal/app/
internal/provider/         # Catalog, Search, Playback, Account, Interaction, History 接口
internal/provider/mock/
internal/provider/authorized/  # 没有合作凭据和官方规范前只有空适配器/错误门控
internal/auth/             # AccountSession, AuthState
internal/catalog/
internal/playback/         # MediaResolver; playback state; transport; diagnostics
internal/comment/
internal/danmaku/
internal/library/
internal/history/
internal/storage/sqlite/
internal/secrets/          # Windows DPAPI / macOS Keychain
internal/settings/
internal/update/
internal/observability/
frontend/src/{views,components,layouts,router,stores,styles,mocks,bindings}/
tests/{unit,e2e,fixtures}/
```

## 关键边界
1. 所有远端服务通过 Go Provider Interface；每个功能有 capability gate，返回 `unsupported`, `not_authorized`, `unavailable`, `ready`。
2. Provider 不得隐式将用户账号资料发送给第三方服务器；日志不得写 Cookie、Token、手机号、完整视频播放 URL。
3. `AuthService` 不能由普通前端持久化账户凭据；认证令牌需操作系统安全存储。
4. `Storage` 用于本地剧集信息、收藏、播放位置、搜索历史、任务状态；迁移必须可回滚/可备份。
5. 观看历史 Cloud Sync 在官方接口未就绪时禁用，并显示“本地保存”；不制造已经同步的假象。
6. 收藏冲突规则：账户级数据优先由用户确认合并；不可静默覆盖；幂等键防止多次发送。
7. 播放器包括播放源状态机：Idle → Resolving → Ready → Playing/Paused → Ended/Error；Seek、切集、关闭窗口要取消过期请求。
8. 如媒体受 DRM 保护且没有合法许可，显示无法播放与合法来源提示；不以规避 DRM 为开发任务。

## 本地表（草案）
- `dramas(id, source, title, cover, genres_json, episode_count, updated_at)`
- `episodes(id, drama_id, idx, duration_ms, title, media_ref)`
- `history(id, account_scope, drama_id, episode_id, position_ms, duration_ms, watched_at, dirty)`
- `favorites(id, account_scope, drama_id, created_at, dirty)`
- `search_history(id, query, searched_at)`
- `settings(key, value_json, updated_at)`
- `provider_caps(provider, capability, status, checked_at, reason)`
- `sync_queue(id, kind, payload_json, idempotency_key, state, retry_count, created_at)`
- `schema_migrations(version, applied_at)`

## 前端视图与路由
`/home`, `/search`, `/drama/:id`, `/watch/:dramaId/:episodeId`, `/favorites`, `/history`, `/login`, `/settings`。评论为 Watch 的抽屉状态，弹幕设置为 Watch 子面板且可通过 `/watch/...?...` 直达以保证独立验收。每张 PNG 都有独立 Story、截图与组件测试。

## 非功能性工程门禁
- `go test ./...`, `npm run typecheck`, `npm run test`, `npm run build`, `npm run test:e2e` 全绿
- 增加失效请求取消、重试退避、播放质量切换、缓存上限和故障诊断
- 代码评审检查屏幕截图差异、安全存储、错误提示、第三方许可证
- 发行构建要分别在 macOS/Windows runner 做 smoke test；未执行不能声称完成。
