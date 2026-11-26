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

在 `frontend` 目录运行 Playwright 时，可指定端口来使用独立的预览服务。默认模式会在启动预览前构建前端。运行过 `npm run build` 后，可以用下列命令直接预览这份产物。CI 也使用此模式。此模式会启动自己的预览服务，端口已占用时测试报错。

macOS shell

```sh
JINDI_E2E_PREBUILT=1 JINDI_E2E_PORT=4175 npm run test:e2e
```

Windows PowerShell

```powershell
$env:JINDI_E2E_PREBUILT = '1'
$env:JINDI_E2E_PORT = '4175'
npm run test:e2e
```

Windows 打包会分别启动安装版和便携版，检查 WebView2 页面就绪、应用版本、amd64 运行架构和正常关闭，再从退出后的 SQLite 副本核对完整性和当前业务结构。在 Windows 桌面会话也可单独运行同一检查，脚本为被测程序创建临时配置目录。未提供 `-NativeProbe` 时，脚本会用 Go 构建探针。

```powershell
pwsh -NoProfile -File scripts/smoke-windows-desktop.ps1 -Application 'C:\Program Files\JindiDrama\JindiDrama.exe' -ExpectedVersion 'v0.1.0'
```

macOS 打包会解压最终 Universal ZIP，在 Apple Silicon 上启动归档中的应用，并确认实际运行的是 arm64。Intel runner 下载同一份产物、核验 SHA-256，再确认 amd64 启动。两边都核对应用版本。正式候选在启动前校验解包后的公证票据，并运行 macOS 分发策略检查。开发者用本机运行测试包时，使用独立的 `local.jindidrama.archiveqa.*` bundle ID。[Apple 代码签名技术说明](https://developer.apple.com/documentation/technotes/tn3161-inside-code-signing-certificates)

GitHub 的测试包在产物名称中标注平台和轮次，例如 `package-windows-latest-attempt-2` 和 `package-macos-latest-attempt-2`。下载复验会核对本轮的版本、提交和 SHA-256。遇到构建或验收失败时，选择「重新运行所有任务」或发起新运行，使两平台都能生成当前轮次的产物。

### 准备 GitHub Release

#### 签名环境

在 GitHub 仓库的「Settings → Environments」设置两个环境。先在每个环境的部署分支和标签规则中，仅允许已审核的发行分支或标签，再将签名凭据保存在对应环境的 Environment secrets 中。签名凭据应只配置在对应环境。

| 环境 | 凭据 |
|---|---|
| `release-windows` | `WINDOWS_CERT_PFX`、`WINDOWS_CERT_PASSWORD` |
| `release-macos` | `MACOS_CERT_P12`、`MACOS_CERT_PASSWORD`、`MACOS_SIGN_IDENTITY`、`APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD` |

环境可按维护者安排要求人工审核。选择 `smoke` 时使用普通测试任务，无需配置签名环境。[GitHub 环境与保护规则](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)

#### 生成并核对交付包

正式打包检查所选提交的完整 Git 历史。可先运行 `bash scripts/verify-distribution-source.sh`，再按照验收记录准备发行。

1. 在拟发布提交上以 `release` 模式运行 `package` 工作流。签名、公证、双平台验收和下载复验全部通过后，会产生 `release-handoff-<版本>-attempt-<轮次>`。如果一轮失败，重跑全部任务或重新发起打包，使两个平台产物来自同一轮。下载交付包并解压到干净目录。
2. 在本地仓库准备指向构建提交的拟发布标签。运行 `node scripts/package-checksums.mjs --verify-release-tag v0.1.0 "<解压目录>"`，将 `v0.1.0` 替换为拟发布标签。命令校验交付包内全部文件的 SHA-256，以及发行说明中的版本和构建提交。核对说明中的 GitHub 打包运行和轮次后再发布。
3. 在 GitHub 上准备指向构建提交的版本标签。用 `RELEASE-NOTES.md` 和交付包内全部文件建立 GitHub Release 草稿。预发布标签须标记为预发布。[GitHub Release 管理](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository)
4. 用已登录的 GitHub CLI 查询草稿 ID，再核对草稿正文、上传资产以及远程标签实际指向的构建提交。示例中的 `123456789`、`v0.1.0` 和目录替换为本次发行值。JSON 保存到交付包目录之外。命令成功后由维护者发布。

```sh
gh api --paginate repos/wuekevin/JindiDrama/releases --jq '.[] | select(.draft) | [.id, .tag_name] | @tsv'
gh api repos/wuekevin/JindiDrama/releases/123456789 > release-draft.json
node scripts/package-checksums.mjs --verify-release-draft v0.1.0 "<解压目录>" release-draft.json
```

#### Windows 证书库签名

GitHub 托管版使用已配置的 PFX 凭据临时导入证书库，签名时按指纹选择证书。使用已有硬件密钥或不可导出私钥的维护者，可在对应 Windows 环境先执行 Wails Windows NSIS 构建，再从项目根目录运行下列命令。需要 PowerShell 7、Windows SDK 的 SignTool、命令路径中支持 `!uninstfinalize` 的 NSIS 3，以及当前用户 `My` 证书库中具有可访问密钥的签名证书。若证书位于本机 `My` 库，添加 `-MachineStore`。

```powershell
pwsh -NoProfile -File scripts/sign-windows-package.ps1 -SigningThumbprint $thumbprint -SignTool $signtool -OutputDirectory build/bin/store-signed
```

脚本在独立目录签署便携程序、卸载器和安装器，校验发布者指纹和时间戳后，将签名的便携程序及安装器一起交付到新目录。原始构建可继续用于测试。[Microsoft SignTool](https://learn.microsoft.com/windows/win32/seccrypto/signtool)

## 短剧分享

详情页可复制短剧链接，播放页和评论页可复制当前分集链接。桌面安装版通过 `jindi:` 协议打开演示目录的短剧和分集，外部打开的分集默认暂停。Windows 便携版可用命令 `JindiDrama.exe "jindi://watch/d1/2"` 打开分集。浏览器预览复制当前站点的可访问地址。

苹果 CMS 详情页也可复制剧目链接。链接只包含片源指纹和剧目 ID。接收方须已启用同一片源，打开后进入短剧详情。

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

## 苹果 CMS V10 授权片源

金帝短剧的 Go Provider 直接请求兼容苹果 CMS V10 的片源。接口参考了两个开源项目的调用方式及字段，其中可直连的 CMS 目录来自 `hongguo-drama` 的自定义片源规范。运行金帝短剧无需安装 Docker，也无需启动任一参考应用。红果官网公开分类、榜单与详情可作字段参考。红果官方搜索、播放、互动与账号能力仍待正式接口联调。

在桌面应用打开「设置 → 播放 → 苹果 CMS 片源」，填写自有或获授权、无需凭据或查询参数的公网 HTTPS API 地址，例如 `https://cms.example.com/api.php/provide/vod/`。点击「测试连接」可读取目录第一页并验证响应，空目录也可以通过。点击「保存片源」会验证目录并把地址写入本机 SQLite。验证失败时保留已保存的地址。保存或清除后，完全退出并重新打开应用即可启用新片源或恢复演示目录。浏览器演示不提供此入口。

应用从该地址读取首页目录、搜索、详情和分集。首页预览目录第一页。点击「查看更多」或搜索关键词后，可在结果区域切换上游页码。继续翻页需要片源返回匹配的当前页码和有效总页数。旧片源仍可显示第一页。片名和简介通过目录读取。顶部「分类」读取片源的分类表，按分类 ID 查询和翻页。若片源没有提供分类表，仍可在该页查看全部目录。剧目提供 `vod_pic` 时，首页、搜索、分类和详情可通过 Go 安全加载静态 JPEG、PNG 或 WebP 封面。封面不可用时会显示占位图。

可播放分集需在 `vod_play_url` 中提供带集号的公网 HTTPS 直链，格式支持 MP4、WebM 和未加密的 HLS 点播。例如 `第1集$https://media.example.com/episode-1.m3u8`。目录支持常见的 `#` 分集与 `$$$` 路线分隔符。HLS 分段使用 MPEG-TS 或 fMP4，清单和媒体服务器需可通过公网 HTTPS 直连。直播、加密和需要登录的链接不在支持范围内。该来源提供浏览和播放。剧目可在详情中收藏。收藏和观看进度保存在本机。打开收藏或观看历史时，应用分批向当前片源核对剧目、封面和可播放分集。首页会核对最近观看的 CMS 剧目，支持继续观看仍可播放的分集。片源停用、切换或剧目移除后，仍可删除对应收藏。片源联网范围见[隐私说明](docs/PRIVACY.md)。

CMS 播放页使用「设置 → 播放 → 播放方式」。播到片尾时，应用先保存本集已看完。选择「连续播放（自动下一集）」时，应用只会核验紧邻的下一集并为它重新取得播放票据。下一集缺失时停在本集，校验或取票失败时可重试，已看完记录会保留。若系统阻止自动播放，可用播放器的播放按钮开始下一集。「单集播放」在片尾停止，「单集循环」会在当前票据有效期内从头重播。直接进入播放页或手动选集后，通过播放器开始播放。

选集下方的「本机评论」可添加和管理本集评论。「本机弹幕」可在当前播放位置添加弹幕，调整字号、透明度和显示区域，并查看本集记录。评论和弹幕按片源、剧目和分集保存在本机 SQLite，保存时不会联系片源。

苹果 CMS 播放使用「设置 → 快捷键」中的播放、快进快退、相邻分集、全屏和弹幕绑定。编辑评论或弹幕时，按键由输入控件处理。

运营方可以用启动环境变量 `JINDIDRAMA_CMS_URL` 指定片源，沿用原有的 API 路径和查询参数支持。环境配置优先于本机设置。托管时，设置页只显示状态，环境地址和参数不会写入 SQLite。若环境配置无效，本次启动使用演示目录并显示错误。取消环境变量后重新启动，应用会读取已保存的本机片源设置。

退出已打开的金帝短剧，再用下面的命令启动。

macOS（终端）

```sh
JINDIDRAMA_CMS_URL='https://cms.example.com/api.php/provide/vod/' '/Applications/JindiDrama.app/Contents/MacOS/JindiDrama'
```

Windows（PowerShell）

```powershell
$env:JINDIDRAMA_CMS_URL = 'https://cms.example.com/api.php/provide/vod/'
& 'C:\Program Files\JindiDrama\JindiDrama.exe'
```
