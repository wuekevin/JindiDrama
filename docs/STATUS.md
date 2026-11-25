# 发行源码验收

本分支用于从独立发行历史构建。应用提供演示目录、授权本地片库、播放、收藏、观看历史、互动、设置和桌面版本更新。

- 从源码提交 `a791627946d6ecf45fc8c7842e74281ed25387b1` 的 bundle 克隆后，Node 22 锁文件安装、前端构建、79 项前端单测和 213 项独立 Playwright 测试通过。Go vet、全量 Go 测试、竞态检查和工作流静态检查通过。
- macOS Universal 测试 ZIP 在 Apple Silicon 解包启动和退出，通过 SQLite 检查。Windows 安装器和便携版构建通过，安装器内主程序与便携版一致。两平台软件文件和文档共 13 项 SHA-256 通过。
- GitHub 当前版本打包、Windows 和 Intel Mac 原生运行、正式签名和公证仍待验收。官方红果 Provider 的启用条件见 [上游授权](UPSTREAM-AUTHORIZATION.md)。打包和验证流程见 [README](../README.md)。

- Windows 证书库签名流程已通过模拟签名和验签、NSIS 卸载器钩子及工作流静态检查。真实证书和 Windows 签名验收仍待托管或实机运行。
