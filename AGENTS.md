# AGENTS.md — 金帝短剧（JindiDrama）

给接手本项目的 AI 编码代理（Codex / Claude Code 等）的说明。先读 `docs/STATUS.md`（唯一的进度与验证记录），再动手。

## Codex 与 BMad
- 从本仓库根目录打开 Codex。项目级技能位于 `.agents/skills/`，运行时位于 `_bmad/`，开发计划与评审记录位于 `_bmad-output/`。
- 用 `bmad status` 查看安装状态，用 `bmad setup` 初始化或刷新运行时，用 `bmad-build` 接手路线图里的单个 Story。已有 PRD、架构与验收记录仍在 `docs/`。
- 首次安装使用 BMad 上游提交 `e435a22574fe55722841086e5566420178b98df3` 的 Core Tools 与 Method 模块，版本为 `6.13.0-next`。

## 项目是什么
Go + Wails v2 桌面应用，前端 Vue 3 + TypeScript + Vite + Pinia + Vue Router(hash) + SCSS。十个页面（首页、搜索、详情、播放、评论、弹幕设置、收藏、历史、登录、设置）按 1448×1086 的产品设计基线实现。本地存储 SQLite（modernc，纯 Go）。默认使用自制演示目录，也支持经配置的授权本地片库和苹果 CMS V10 片源。Go 直接请求配置的 CMS 地址。红果官方内容源仍待联调。

## 目录
- `frontend/` 前端；`frontend/tests/e2e/` Playwright；`src/**/*.spec.ts` Vitest；`frontend/tools/` 截图脚本
- `internal/provider/{mock,licensed,applecms,authorized,hongguoweb,registry}` Provider 适配层；界面只能经 Provider 取数据
- `internal/app` Wails 绑定；`internal/storage/sqlite`、`internal/cache`、`internal/observability`、`internal/update`、`internal/autostart`
- `build/windows/installer/project.nsi` 自定义 NSIS 模板（安装目录 `Program Files\JindiDrama`，快捷方式名「金帝短剧」）
- `.github/workflows/ci.yml`（push 即跑，Windows+macOS）、`package.yml`（手动触发出安装包）
- `docs/` PRD、架构、设计、验收、状态

## 常用命令
```
cd frontend && npm ci
npm run typecheck && npm test && npm run build
npm run preview &            # 127.0.0.1:4173，Playwright 依赖它
npx playwright test          # 全量 e2e（约 1 分钟）
cd .. && go vet ./... && go test ./...
~/go/bin/wails build -s -skipbindings -ldflags "-X jindi-drama/internal/app.Version=v0.1.0"   # CLI 版本 v2.17.1
```
改前端后必须先 `npm run build` 再跑 Playwright（它测的是 dist）。e2e 通过 `window.__JD_MEDIA='sim'` 用模拟画面；`11-p1-playback.spec.ts` 用真实 `<video>`，并行时偶发时序失败，单独重跑可过。

## 硬性约束（不要改）
1. **不实现红果/字节的非公开接口**：设备注册、登录模拟、播放地址解析、评论接口、CENC 解密等一律不做。`HongguoAppProvider`(`authorized`) 与 `HongguoWebProvider` 必须保持禁用桩（所有能力 `not_authorized`，不发网络请求，有测试守护）。启用条件见 `docs/UPSTREAM-AUTHORIZATION.md`：官方书面授权 + 接口文档 + 测试账号。
2. 不得宣称云同步/官方账号已打通；演示账号只是本地会话（验证码 123456）。
3. 状态必须诚实：只有实际跑过的才写 Verified；没跑的写 Not started / Written 未运行 / Blocked。结果记入 `docs/STATUS.md`。
4. 不要提交密钥、证书、cookie、token；日志必须脱敏（`internal/observability`）。
5. 不要引入需要联网才能跑的测试。

## 约定
- 回复/文档用简体中文。
- 视觉改动要截图对照设计稿并记录；浅色主题用 `.on-dark` 作用域保护深色底区域。
- 缩放：`#app` 用 CSS zoom（基线 1.05，范围 0.82–2.2），最小窗口 1280×860，默认 1448×1086。
- 提交信息用中文、一件事一个提交；推送前先本地跑完上面的检查。大文件一次推送可能 HTTP 408，分批推。
- 提交身份（本机未配置全局 git 身份）：`git -c user.name=wuekevin -c user.email=174148816+wuekevin@users.noreply.github.com commit ...`

## 尚未完成（详见 STATUS.md）

- Windows 与 Intel Mac 原生验收、GitHub 当前版本打包、正式签名与公证。
- 官方内容源接入和端到端验证，等待适用的授权、文档和测试环境。
