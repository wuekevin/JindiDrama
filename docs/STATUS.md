# Story 状态（仅记录有真实验证的项；环境：macOS arm64，Chromium 159，Go 1.27，Node 26，Wails CLI 2.17.1）
| Story | 状态 | 证据 |
|---|---|---|
| E00-S01 工程初始化 | Verified（macOS 构建）/ Windows 未验证 | `wails build` 生成 build/bin/JindiDrama.app（仅编译，未做 GUI 冒烟）；`go test ./...` 通过 |
| E00-S02 DESIGN/EXPERIENCE | Verified(文档) | docs/DESIGN.md, docs/EXPERIENCE.md |
| E00-S03 AppShell/基础组件 | Verified | frontend/src/{layouts,components} |
| E01-S01 首页 | Verified | design/diff/01-report.md |
| E02-S01 搜索 | Verified | design/diff/02-report.md |
| E03-S01 详情 | Verified | design/diff/03-report.md |
| E04-S01 播放 | Verified（UI+状态机；默认真实演示媒体，可切模拟） | design/diff/04-report.md |
| E05-S01 评论 | Verified（本地 Mock） | design/diff/05-report.md |
| E06-S01 弹幕设置 | Verified（本地 Mock） | design/diff/06-report.md |
| E07-S01 收藏 | Verified | design/diff/07-report.md |
| E08-S01 历史 | Verified | design/diff/08-report.md |
| E09-S01 登录（演示） | Verified（仅演示） | design/diff/09-report.md |
| E10-S01 设置 | Verified（系统级开关待桌面端） | design/diff/10-report.md |
| E00-S04 十页截图/diff/响应式 | Verified（截图+像素指标+3 档分辨率无横向溢出）；人工逐像素比对为抽查 | design/rendered, design/diff |
| E11-S01 SQLite 模型/迁移/备份 | Verified（Go 单测：WAL、迁移升降级、备份不覆盖、重启保持；App 绑定+白名单） | internal/storage/sqlite, internal/app/storage*.go |
| E11-S01 前端接入 | Verified（桥接单测）；桌面端实际读写 SQLite 未做 GUI 验证 | frontend/src/bindings |
| E11-S02 合法演示媒体播放 | Verified（Chromium 真实 <video>：播放/Seek/倍速/切集/续播/结束重播） | frontend/public/demo（自制 testsrc2 片段），tests/e2e/11-p1-playback.spec.ts |
| E11-S03 失败重试/取消过期请求/播放源开关 | Verified（指数退避 0.5s/1s 后报错，手动重试恢复；快速切集只保留最后一次） | 同上 |
| E11-S03 缓存上限（LRU）/脱敏故障日志 | Verified（Go 单测：淘汰、路径穿越拒绝、缩容、日志脱敏与滚动；前端桥接单测+设置页 e2e）；尚未接入真实下载流程 | internal/cache, internal/observability |
| E11-S04 CI 双平台构建 | Written, **未运行**（.github/workflows/ci.yml，未推送到 GitHub，Windows 从未执行） | — |
| E11-S04 托盘/通知/自动更新/签名 | Not started（Wails v2 无内置托盘；签名需证书） | — |
| P2 红果官方接入 | Blocked（无官方授权，见 API-LEGAL-GATES G0） | — |
| P3 Windows 构建/签名/公证 | Not started（Windows 与 macOS 签名均未执行） | — |

## 桌面冒烟（macOS arm64，2026-11-23）
- Verified：`build/bin/JindiDrama.app` 能启动，窗口渲染真实首页；自动创建 `~/Library/Application Support/JindiDrama/{jindi.db,cache/,logs/}`；迁移 1、2 已应用；退出后 WAL 合并，`integrity_check` = ok。
- 未验证：窗口内点击收藏/历史后重启仍保留（未做 GUI 自动化，仅有单测与桥接测试）；Windows 全部；CI 未运行。

## CI 实跑结果（2026-11-24，run 45285176749）
- Verified：windows-latest 与 macos-latest 均通过 typecheck、Vitest、build、Playwright e2e、go vet/test、wails build（产物存在）。
- 过程中修了两处：workflow YAML 语法错误；CI 的 wails CLI 由 2.10.2 升至 2.17.1（Info.plist 模板需要）。
- 仍未验证：Windows 桌面窗口实机运行、签名/公证、托盘/通知/自动更新。

## E11-S04 后续（2026-11-24）
- Verified（单测 + CI）：检查更新（`internal/update`，GitHub Releases，semver 比较、404=暂无发布、坏响应/网络错误、链接白名单）；设置页接入真实检查（浏览器预览如实提示不支持）；系统通知（beeep）仅在发现新版本时触发；版本号经 ldflags 注入。仓库尚无 Release，因此「发现新版本」路径只有单测，没有真实端到端。
- Written, 未运行：`package.yml` 的 macOS 签名/公证与 Windows signtool 步骤——仅在配置了对应 secrets 时执行，目前没有证书，均被跳过；产物仍为未签名。Windows 签名只签安装器 exe，不含安装器内部的主程序 exe。
- Blocked：签名/公证需要 Apple Developer 账号与 Windows 代码签名证书。
- Not started：系统托盘（Wails v2 无内置支持，需引入第三方托盘库并与 Wails 事件循环共存，风险高）；自动静默更新（签名前不做，只引导用户到 Release 页下载）。
- Blocked：桌面 GUI 持久化冒烟——AppleScript 无辅助功能权限，无法自动点击；需用户授权或手动验证。

## 桌面 GUI 持久化冒烟（2026-11-24，macOS arm64）
- Verified：在运行的 `.app` 中点击收藏 → `favorites` 表写入 1 行；退出重开后「我的收藏」页显示该剧（数据来自 SQLite）；点「取消收藏」后表回到 0 行。
- 方法：Swift/Quartz 取窗口位置并发送鼠标事件（AppleScript/System Events 会超时，原因未查明）。
- 未验证：历史记录、设置项的 GUI 重启保留；Windows 实机。

## GUI 冒烟补充与缺陷修复（2026-11-24）
- Verified（桌面 `.app` GUI）：收藏、观看历史重启后保留；设置项（主题）写入 `settings` 表。
- 修复 1：主题/缩放/字体只在打开设置页时才应用，重启后首页恢复默认。现在 `main.ts` 启动即实例化 appSettings；新增 e2e 回归测试。
- 修复 2：浅色主题从未目视验收，侧栏/轮播/卡片/播放器文字在浅色下不可读。新增 `.on-dark` 作用域覆盖深色底区域，并逐页截图复核（首页、详情、播放、收藏、登录、设置）。搜索、历史两页的浅色未逐张复核。
- 已知缺口：设置里「关闭窗口时：最小化到托盘」「开机启动」只保存选项，桌面端没有托盘、也没有真正注册开机启动，不能算已实现。
