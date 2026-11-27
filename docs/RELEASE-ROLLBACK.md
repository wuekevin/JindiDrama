# 桌面发行回滚

当旧版应用支持的数据库版本低于当前版本时，先为旧版生成兼容副本，再启动旧版。历史提交 `f97bd81afd803e530d91dbc7bc28fe4b88d99e39` 使用 SQLite v2，当前源码使用 v10。其他发行版的目标版本以对应源码或发行记录为准。

1. 确认新版已补写待保存的离线记录，再退出应用。回滚工具转换 SQLite 中已提交的数据。从用户配置目录备份现有数据库和同名 `-wal`、`-shm`、`-journal` 文件（若存在）。保留这一组作为恢复新版的资料。
2. 使用已支持回滚命令的**当前**桌面程序生成新的副本，再安装旧版。工具读取已提交的 SQLite 内容，在独立副本上降级并检查完整性。输出目录使用支持硬链接的本地文件系统，例如 APFS 或 NTFS。以下示例生成 v2 副本，其他发行版按实际目标版本修改。

macOS 在终端运行。安装到其他位置时，替换程序路径。

```sh
"/Applications/JindiDrama.app/Contents/MacOS/JindiDrama" --prepare-rollback \
  -source "$HOME/Library/Application Support/JindiDrama/jindi.db" \
  -output "$HOME/jindi-v2.db" -target 2
```

Windows 在 PowerShell 运行。按用户安装时，默认程序路径为 `$env:LOCALAPPDATA\Programs\JindiDrama\JindiDrama.exe`。使用便携版或自选安装目录时，将 `$application` 改为实际程序路径。[PowerShell 启动进程参数](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/start-process#-argumentlist)

```powershell
$application = "$env:ProgramFiles\JindiDrama\JindiDrama.exe"
$source = "$env:APPDATA\JindiDrama\jindi.db"
$output = "$env:USERPROFILE\jindi-v2.db"
$rollbackArgs = '--prepare-rollback -source "{0}" -output "{1}" -target 2' -f $source, $output
$process = Start-Process -FilePath $application -ArgumentList $rollbackArgs -Wait -PassThru
if ($process.ExitCode -ne 0) { throw "生成回滚副本失败，退出码 $($process.ExitCode)" }
Get-Item -LiteralPath $output
```

如果当前程序尚无此命令，可在当前版本的源码目录中使用 Go 工具。

```sh
go run ./cmd/prepare-rollback -source "/path/to/JindiDrama/jindi.db" -output "/path/to/jindi-v2.db" -target 2
```

macOS 从源码运行时，请使用启用了 CGO 的 Go 工具链。工具需要系统 ACL 支持来保护暂存目录。也可直接用当前版桌面应用执行回滚。

3. 仍在退出状态时，将原 `jindi.db` 及其辅助文件一起移到单独目录。随后将验证过的副本复制到原位置并命名为 `jindi.db`。确认该文件名下没有遗留的辅助文件，再启动旧版。
4. 打开收藏、观看历史和设置，核对收藏排序、精确进度和关键设置。清点成功后保留原库直到回滚结束。如果恢复到新版，先退出并单独保留回滚期间使用的数据库，再将原库及属于它的辅助文件作为一组恢复。

v2 能读取收藏顺序、观看进度、搜索历史和旧版认识的设置。v3 起加入的本地弹幕、评论和补写回执保留在原库。目标版本低于 v7 时，副本保留分集及进度，观看时的总集数保留在原库。目标版本低于 v8 时，已提交的本地评论仍可读取，补写标识保留在原库。目标版本为 v3 至 v8 时，已提交的本地弹幕仍可读取，弹幕重试标识保留在原库。v9 副本保留弹幕回执，v10 使用的数据库暂存标识保留在原库。回滚期间写入旧版副本的改动需要单独保留，两套记录不会自动合并。

默认数据目录为 macOS 的 `~/Library/Application Support/JindiDrama` 和 Windows 的 `%APPDATA%\JindiDrama`。如果路径含有空格，请保留命令中的引号。

工具在私有目录校验源数据库和持久辅助文件的副本，再应用降级迁移。若源文件在复制期间发生变化，需退出应用后重试。通过完整性检查后才生成目标文件。
