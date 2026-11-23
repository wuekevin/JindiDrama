# 直接发送给 Claude Code / BMAD Master 的全项目总指令

你担任本项目 BMAD Master / 项目负责人。当前目录包含之前生成的 **10 张独立 PC 客户端 UI 参考图**，以及 PRD、架构、Story 与能力边界文档。任务不是写建议书，而是按 BMAD 工作流将项目实际推进到可运行、可测试、可交付。

## 绝对优先级：先完成 UI，再做业务
**立即开始 P0，先完成 Design System、01 首页，再依次完成其它 9 页；不得先开发红果非公开 API。**

1. 先读取 `README.md`、`docs/PRD.md`、`docs/ARCHITECTURE.md`、`docs/ROADMAP-AND-STORIES.md`、`docs/UI-ACCEPTANCE.md`、`docs/API-LEGAL-GATES.md`。
2. **逐张打开** `design/reference/01-首页推荐.png` 至 `10-应用设置.png`。这十张图片才是主要视觉依据，不得仅根据文本猜想风格。
3. 使用当前项目已安装的 BMAD 技能/工作流，先列出真正可用的命令；优先调用 `bmad-help` 或已安装的 BMAD Master。不要假设旧版 slash 命令一定存在，也不要反复安装覆盖用户配置。
4. 由 Analyst/PM 输出或校验 PRD；由 UX 角色输出 DESIGN.md/EXPERIENCE.md（或当前版本等价文件），由 Architect 生成技术约束和 ADR；由 Scrum Master 将此包 Story 导入当前版本 BMAD 的迭代跟踪。
5. 使用 Claude Frontend Design（如当前环境可用）承担真实 Vue 界面设计；Developer 负责代码与测试，QA 负责独立截图验收。Agent 可按任务选择模型，但必须使用实际可用机制，不能声称未执行的切换。

## 技术不可改动项
- Go + Wails v2，Vue 3 + TypeScript + Vite，Pinia，Vue Router，SCSS/CSS variables，Lucide。
- Windows 11 为首发，macOS arm64/x64 第二目标。原生安装和 system integration；不使用 Electron、不使用 Docker。
- Go 必须维持清晰模块划分：Provider、Auth、Catalog、Playback、Danmaku、Comment、History、Library、SQLite Store、Secrets、Settings、Update。
- 应用初期须在完全离线 Mock 模式仍能演示所有页面与按钮；系统不依赖自建服务器、MySQL、Redis 才能启动。
- 包含独立 `MockProvider`、`LicensedDemoProvider`、`AuthorizedHongguoProvider`；最后一个没有正式合作授权时必须保持未实现/禁用状态，不得擅自模拟官方手机用户或宣称打通云同步。

## UI 还原要求
- 参考尺寸 1448×1086，先以此为截图比较基准，再适配 1366×768、1920×1080、2560×1440。
- 禁止将 PNG 整张贴在页面背景中。应建立真正可复用的 HTML/CSS/Vue 组件及键盘、鼠标交互。
- 允许对字体和海报素材按授权情况用相似资产替换，必须记录偏差；设计图中戏名、人物与数据均为示例。
- 每个页面必须单独作为一个 Story，实现并验收后才进入下一个页面；评论与弹幕设置可以是播放器的独立侧栏状态，但必须可独立进入与截图。
- 建立页面导航、加载/空态/错误、悬停/选中/禁用、表单校验、窗口缩放与键盘操作。

## 页面实施顺序（10 页，严格逐页）
01 首页推荐、02 搜索结果、03 短剧详情、04 视频播放、05 评论互动、06 弹幕设置、07 我的收藏、08 观看历史、09 账号登录、10 应用设置。

## 编程与测试门禁
- 每个 Story 必须至少包含：功能实现、Vue 单测或 Go 单测、Playwright 操作测试、1448×1086 实际截图和原图差异报告。
- 每页完成后将截图保存 `design/rendered/XX-*.png`，差异记录在 `design/diff/XX-report.md`，并截图复核修正至少一轮；如果测试环境缺少截图工具必须记录 blocked，并提供准确安装步骤，不能伪称通过。
- 整体测试：`go test ./...`、`npm run typecheck`、`npm run test`、`npm run build`、`npm run test:e2e`，实际命令根据创建工程调整，失败时修复而不是跳过。
- 状态记录必须准确：Not started / In progress / Blocked / Verified。无真实验证的模块不可写 Verified。

## 业务接入阶段（P0 完成后再做）
P1：本地 SQLite 观看历史、收藏、配置、合法演示媒体播放、断点续播、播放器状态机。
P2：在拿到官方授权和技术文档后实现真正的红果官方账号、推荐/搜索、正式播放、弹幕、评论及云同步；无授权时仅保留接口与 Mock Provider，阻塞 P2 不阻碍 UI 和合规演示交付。
P3：Windows/macOS 构建、原生安装包、自动更新、签名、公证（如具备证书）、安全和版权审查、发行回归。

## 继续开发规则
- 不要在未完成首个 Story 的情况下反复重写 PRD 或给出高层建议。
- 每个阶段完成都汇报文件路径、实际执行命令、测试结果、截图位置、未解决的阻塞，继续按 Story 推进。
- 不要声称自己已经运行代码或构建 Windows/macOS 安装包，除非工具实际执行并返回成功。
- 不得绕过 DRM、提取用户秘密、复制闭源客户端、伪造官方授权、将非公开接口视为可自由商用。
- 优先使用现有项目；如已有演示代码，先评估可复用性，不合格的单文件 UI 可重构，不要盲目套用。

## 现在马上执行的实际任务
A. 检查 10 张 PNG 与其尺寸；检查当前项目代码及 BMAD 安装状态。
B. 创建 BMAD 可追踪的 Epic/Story 与设计系统文档。
C. 初始化或修复 Wails/Vue3/TS 工程，建立可运行的 AppShell。
D. 完成 `01-首页推荐.png` 的真实 Vue 代码、Mock 数据、截图对比、修正与测试报告。
E. 通过 G-UI-01 后进入 02，直至 10。所有步骤均应落实到源码与文件中，不接受仅返回计划。
