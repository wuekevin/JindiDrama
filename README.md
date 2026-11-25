# 金帝短剧

项目	最终命名
中文名称	金帝短剧
英文名称	Jindi Short Drama（暂定）
项目代号	jindi-drama
Windows 程序	JindiDrama.exe
安装程序	JindiDrama-amd64-installer.exe
macOS 应用	JindiDrama.app
本地数据目录	JindiDrama
Go Module	jindi-drama（暂定）
UI 标题	金帝短剧

金帝短剧是 Windows 和 macOS 桌面应用，支持离线演示和授权本地片库。官方服务的启用条件见上游授权说明。

## 接续开发

从仓库根目录打开 Codex，使用 `bmad status` 查看项目级 BMad 安装，使用 `bmad-build` 接手路线图中的故事。以 `docs/STATUS.md` 为验收记录。

## 文档
- `docs/PRD.md`：范围、功能、验收、边界
- `docs/ARCHITECTURE.md`：技术栈、模块设计、数据模型、接口适配
- `docs/ROADMAP-AND-STORIES.md`：阶段、Epic、Story 与验收定义
- `docs/UI-ACCEPTANCE.md`：十页逐页视觉/交互清单
- `docs/UPSTREAM-AUTHORIZATION.md` 说明官方上游的启用条件
- `contracts/provider-contracts.ts`：前端 Provider 类型定义草案
- `contracts/api-capabilities.json`：功能门禁与降级状态

## 特别注意
应用截图展示演示数据。影视海报、人物、视频和其他媒体素材须为原创或取得相应授权。正式上游能力须经授权和端到端验收后启用。

## 隐私与第三方许可

设置页的「隐私与反馈」在离线时也可查看。打包产物随程序提供 `PRIVACY.md`、`FRONTEND-LICENSES.md` 和 `GO-LICENSES.md`。双平台正式交付包的许可文件按平台命名，分发时与安装包一并保留。内置演示媒体来源见 `frontend/public/demo/README.md`。

## 开发与验证（P0 已实现）
```
cd frontend && npm ci
npm run dev            # 浏览器预览 http://localhost:5173
npm run typecheck && npm run test && npm run build && npm run test:e2e
cd .. && go test ./...  # 需先 npm run build 生成 frontend/dist（main.go 内嵌）
wails build -s -skipbindings  # Windows 本机原生构建
```
macOS 支持 Ventura 13.0 或更新版本，可在 Apple Silicon 或 Intel Mac 上安装。macOS 本机原生构建使用对应部署目标。

```sh
CGO_CFLAGS=-mmacosx-version-min=13.0 CGO_LDFLAGS=-mmacosx-version-min=13.0 wails build -s -skipbindings
```


## 桌面更新渠道

「设置 → 关于与更新」默认检查稳定版。主动选择「预发布」后会同时查找有效的预发布版和稳定版，用于提前试用。切回稳定版会撤下预发布下载入口。GitHub 提供匹配平台的安装包及 SHA-256 时，设置页可校验下载。也可打开项目发布页。安装由用户手动完成。

维护者创建预发布时，使用 `v1.2.3-rc.1` 形式的标签，并在 GitHub Release 中标记为预发布。稳定版使用正式版本标签。打包版本最多 128 个字符，以便启动验收核对完整版本。

回滚时可按 [桌面发行回滚](docs/RELEASE-ROLLBACK.md) 为旧版生成独立的 SQLite 兼容副本。

## 发行产物

打包 GitHub artifact 中包含发行说明、系统要求、来源提交和各平台的 SHA-256 清单。将同一平台的文件解压到同一目录，按随附的 `RELEASE-NOTES.md` 校验后再安装。

打包使用 `npm ci` 安装前端锁文件中的依赖，校验 Go 模块后由 Wails 打包已构建的前端。依赖变更应先更新并提交对应的锁文件。

如需从另一份本地检出独立运行 Playwright，可在 `frontend` 目录执行 `JINDI_E2E_PORT=4175 npm run test:e2e`。CI 和显式指定端口的测试各自启动预览服务。

Windows 打包会分别启动安装版和便携版，检查 WebView2 页面就绪、应用版本、amd64 运行架构和正常关闭，再从退出后的 SQLite 副本核对完整性和当前业务结构。在 Windows 桌面会话也可单独运行同一检查，脚本为被测程序创建临时配置目录。未提供 `-NativeProbe` 时，脚本会用 Go 构建探针。

```powershell
pwsh -NoProfile -File scripts/smoke-windows-desktop.ps1 -Application 'C:\Program Files\JindiDrama\JindiDrama.exe' -ExpectedVersion 'v0.1.0'
```

macOS 打包会解压最终 Universal ZIP，在 Apple Silicon 上启动归档中的应用，并确认实际运行的是 arm64。Intel runner 下载同一份产物、核验 SHA-256，再确认 amd64 启动。两边都核对应用版本，正式候选还校验解包后的公证票据。开发者用本机运行该检查时，测试构建使用独立的 `local.jindidrama.archiveqa.*` bundle ID。

### 准备 GitHub Release

正式打包检查所选提交的完整 Git 历史。可先运行 `bash scripts/verify-distribution-source.sh`，再按照验收记录准备发行。

1. 在拟发布提交上以 `release` 模式运行 `package` 工作流。签名、公证、双平台验收和下载复验全部通过后，会产生 `release-handoff-<版本>-attempt-<轮次>`。如果一轮失败，重跑全部任务或重新发起打包，使两个平台产物来自同一轮。下载交付包并解压到干净目录。
2. 在仓库中运行 `node scripts/package-checksums.mjs --verify release "<解压目录>"`。核对 `RELEASE-NOTES.md` 中的版本、提交、打包运行和轮次。发行标签应指向说明中的同一个提交。
3. 用 `RELEASE-NOTES.md` 和交付包内全部文件建立 GitHub Release 草稿。预发布标签须标记为预发布。确认草稿的资产名称和 SHA-256 后，由维护者发布。[GitHub Release 管理](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository)

## 短剧分享

详情页可复制短剧链接，播放页和评论页可复制当前分集链接。桌面安装版通过 `jindi:` 协议打开演示目录的短剧和分集，外部打开的分集默认暂停。Windows 便携版可用命令 `JindiDrama.exe "jindi://watch/d1/2"` 打开分集。浏览器预览复制当前站点的可访问地址。

## 桌面本地片库

将自有或已获准播放的素材放入同一目录，并在根目录添加 UTF-8 的 `catalog.json`。每部短剧的标识须以 `local-` 开头并在清单中唯一。`episodes` 按集号排列，每集允许一个 MP4、一个 WebM，或同时提供两种格式。路径相对清单目录，子目录使用正斜杠。

```json
[
  {
    "id": "local-landscape",
    "title": "南风纪事",
    "summary": "本地风景短片",
    "license": "作者自有，允许播放和保存副本",
    "allowExport": true,
    "tags": ["风景"],
    "episodes": [["clips/01.mp4", "clips/01.webm"], ["clips/02.mp4"]]
  }
]
```

在桌面应用点击顶栏「片库」，使用「选择目录」保存配置。应用会检查清单和所列媒体是否可打开。重启后会显示新的片库。可在同一处更换或移除配置，移除配置会保留目录中的文件、收藏和观看历史。修改清单后需重新启动应用。为同一部短剧保留原标识，收藏和观看进度会继续关联到它。本地片库使用设置中的倍速和播放方式。

运营方也可在启动时设置 `JINDIDRAMA_LICENSED_MEDIA_DIR`。该配置优先生效，片库界面会显示目录由运营方管理。

拥有复制许可时，可为对应短剧设置可选的 `allowExport: true`。重启后打开分集即可通过「保存本集」复制原文件，同时提供两种格式时保存 MP4，仅提供 WebM 时保存 WebM。缺省或设置为 `false` 时隐藏导出入口。

macOS（终端）

```sh
JINDIDRAMA_LICENSED_MEDIA_DIR="/path/to/media" "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama"
```

Windows（PowerShell）

```powershell
$env:JINDIDRAMA_LICENSED_MEDIA_DIR = 'C:\Media\JindiDrama'
& 'C:\Program Files\JindiDrama\JindiDrama.exe'
```
