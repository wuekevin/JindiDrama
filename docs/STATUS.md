# 发行源码验收

本分支用于从独立发行历史构建。应用提供演示目录、授权本地片库、播放、收藏、观看历史、互动、设置和桌面版本更新。

- 从源码提交 `a791627946d6ecf45fc8c7842e74281ed25387b1` 的 bundle 克隆后，Node 22 锁文件安装、前端构建、79 项前端单测和 213 项独立 Playwright 测试通过。Go vet、全量 Go 测试、竞态检查和工作流静态检查通过。
- macOS Universal 测试 ZIP 在 Apple Silicon 解包启动和退出，通过 SQLite 检查。Windows 安装器和便携版构建通过，安装器内主程序与便携版一致。两平台软件文件和文档共 13 项 SHA-256 通过。
- GitHub 当前版本打包、Windows 和 Intel Mac 原生运行、正式签名和公证仍待验收。官方红果 Provider 的启用条件见 [上游授权](UPSTREAM-AUTHORIZATION.md)。打包和验证流程见 [README](../README.md)。

- Windows 证书库签名流程已通过模拟签名和验签、NSIS 卸载器钩子及工作流静态检查。真实证书和 Windows 签名验收仍待托管或实机运行。

## 本地片库收藏与历史可用状态（2026-11-25）

- 本地媒体不可用时，收藏和历史提供「查看片库」。原分集超出当前目录时提供「选集」。这两个入口保留原进度及收藏、删除操作。恢复目录后的浏览器真实视频测试从原第 2 集 12 秒继续。
- Node 22 类型检查和生产构建、Vitest 79 项、Playwright 213 项、Go vet 和全量 Go 测试通过。macOS Apple Silicon 原生验证无目录、短目录和恢复第 2 集，退出后的隔离 SQLite 保留原进度且完整性为 `ok`。Windows amd64 生产交叉编译通过，WebView2 原生验收待运行。
- 1448×1086 实际界面复核了操作位置、原集数和进度显示。见 [收藏](../design/rendered/07-本地媒体不可用.png) 和 [历史](../design/rendered/08-本地媒体不可用.png)。
