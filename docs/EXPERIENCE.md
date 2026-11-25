# EXPERIENCE.md — 导航与状态矩阵
路由：/home /search /drama/:id /watch/:dramaId/:episodeId /favorites /history /login /settings。
状态：所有数据页支持 loading(骨架)/empty/error(重试)，可用 `?state=loading|empty|error` 触发演示。
交互：Esc 关闭抽屉/弹层/建议；←/→ 切换 Hero；悬停 hover、选中 aria-current/selected、禁用 disabled。
数据：Mock 离线；收藏/历史存 localStorage（P1 迁移到 SQLite）。登录为演示模式，不宣称官方登录。
