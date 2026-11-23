# BMAD Epics / Stories / 里程碑

> `Story ID` 应按当前安装的 BMAD 流程导入或重建；下列编号是项目稳定标识，不代表官方命令名。

## P0: 视觉可验收的可运行 UI（必须先做）
- E00-S01 初始化 Wails + Go + Vue3+TS+Vite+Pinia+Router 工程；检查 Windows/macOS 工具链，可本地启动。
- E00-S02 逐页解析 10 PNG，输出 DESIGN.md（色彩、字体、布局、控件及资产清单）及 EXPERIENCE.md（导航、状态、交互矩阵）。
- E00-S03 实现 AppShell、Sidebar、Topbar、Design Tokens、通用 Button/Card/Modal/Drawer/Toast/Empty/Loading。
- E01-S01 首页：Hero、分类、今日上新、排行榜、继续看，UI 对比截图合格。
- E02-S01 搜索页：搜索、建议、热门词、排序筛选、结果态。
- E03-S01 详情页：简介、演员、选集、收藏、分享、相关推荐。
- E04-S01 播放页：交互 Mock、弹出侧栏、播放器状态机。
- E05-S01 评论抽屉：分页、回复、点赞、输入（模拟）。
- E06-S01 弹幕设置：可见弹幕轨道、参数、过滤和发送（模拟）。
- E07-S01 收藏页：本地收藏 + 批量管理。
- E08-S01 历史页：时间分组 + 续播 + 批量操作。
- E09-S01 登录页：验证码/扫码模拟状态机、安全与错误处理。
- E10-S01 设置页：主题/播放器/缓存/快捷键/更新模拟。
- E00-S04 完成 10 页固定分辨率截图、diff、键盘/响应式检查。

## P1: 本地可靠性 + 可用媒体
- E11-S01 SQLite 数据模型、迁移、备份；离线收藏、历史、设置。
- E11-S02 MediaProvider 接口、合法测试媒体、分集播放、Seek/切集/断点续播。
- E11-S03 失败重试、取消过时请求、画质偏好、故障日志、缓存边界。
- E11-S04 桌面窗口、深链（若实际需求）、更新签名/托盘/通知适配；Windows smoke test。

## P2: 正式服务接入（受授权门槛控制）
- E12-S01 获取官方接入范围/接口文档/沙箱/授权证明，记录阻塞和风险。
- E12-S02 AuthorizedProvider：目录/搜索/详情/播放；正式 capability gate。
- E12-S03 AccountProvider：授权登录、会话恢复/退出、系统 Keychain/DPAPI。
- E12-S04 InteractionProvider：评论和弹幕读写、用户提示、重放保护。
- E12-S05 SyncProvider：收藏/历史/进度对账、冲突处理与降级。
- E12-S06 与官方手机端对照验收、限流、异常/地域测试。

## P3: 发布与质量
- E13-S01 Windows installer + portable；CI 签名、hash、回归。
- E13-S02 macOS arm64/x64 构建、公证/签名路径、运行回归。
- E13-S03 隐私声明、许可证清单、媒体授权检查、日志脱敏。
- E13-S04 灰度更新、回滚、崩溃诊断、发行说明。

## 强制 Gate
G-UI：10 个页面均有截图与交互证据；未完成不得进入上游集成。
G-LOCAL：Mock 模式能独立运行且关键本地数据重启后保持。
G-AUTH：官方明确授权第三方桌面客户端及对应业务能力；否则 P2 保持 blocked，但 P0/P1 可以发布合规 Demo。
G-RELEASE：合法内容来源、技术测试和许可证审查通过。

## Scrum 周期执行模版
每个 Story 必须经历：Ready → Dev → Unit/E2E → Screenshot comparison → Code Review → QA → Done。缺少对应截图或执行证据时标记 Incomplete / Blocked，不接受“代码看起来正确”作为验收结论。
