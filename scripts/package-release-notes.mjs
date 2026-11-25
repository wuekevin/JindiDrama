import { execFileSync } from 'node:child_process'
import { copyFile, lstat, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { hostedPackageHeader } from './package-provenance.mjs'

const [directory, platform = process.platform] = process.argv.slice(2)
const { PACKAGE_VERSION: version, PACKAGE_MODE: mode, GITHUB_SHA: sha,
  GITHUB_ACTIONS: inActions } = process.env
if (!directory || !['darwin', 'win32'].includes(platform) || !['smoke', 'release'].includes(mode) || !version) {
  throw new Error('Expected PACKAGE_VERSION, PACKAGE_MODE, package directory and darwin or win32 platform')
}
execFileSync('go', ['run', './cmd/validate-package-version', version])
if (inActions !== 'true' && mode === 'release') {
  throw new Error('Release-candidate notes require a GitHub Actions build')
}

let header = ['# 金帝短剧 ' + version, '', '- 打包类型 测试包', '- 构建来源 本地工作树（供测试）']
if (inActions === 'true') {
  const head = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
  if (head !== sha) {
    throw new Error('GitHub build provenance does not match the checkout')
  }
  header = hostedPackageHeader(process.env)
}

const onMac = platform === 'darwin'
const files = onMac
  ? [['JindiDrama-macos-universal.zip', '包含 Apple Silicon 和 Intel 版本的 macOS 应用']]
  : [
      ['JindiDrama-amd64-installer.exe', 'Windows 安装程序'],
      ['JindiDrama.exe', 'Windows 便携程序'],
    ]
const sidecars = ['PRIVACY.md', 'FRONTEND-LICENSES.md', 'GO-LICENSES.md']
for (const name of [...files.map(([name]) => name), ...sidecars]) {
  const stat = await lstat(join(directory, name))
  if (!stat.isFile() || stat.size === 0) throw new Error('Missing package file: ' + name)
}
function verifyRuntimeVersion(binary) {
  const report = JSON.parse(execFileSync('go', ['version', '-m', '-json', binary], { encoding: 'utf8' }))
  const flags = report.Settings?.find((item) => item.Key === '-ldflags')?.Value
  if (!flags?.split(/\s+/).includes('jindi-drama/internal/app.Version=' + version)) {
    throw new Error('Executable does not contain the expected application version: ' + binary)
  }
}
if (onMac) {
  const temp = await mkdtemp(join(tmpdir(), 'jindi-package-version-'))
  try {
    const executable = join(directory, 'JindiDrama.app', 'Contents', 'MacOS', 'JindiDrama')
    for (const arch of ['arm64', 'x86_64']) {
      const slice = join(temp, arch)
      execFileSync('lipo', [executable, '-thin', arch, '-output', slice])
      verifyRuntimeVersion(slice)
    }
  } finally {
    await rm(temp, { recursive: true, force: true })
  }
} else {
  verifyRuntimeVersion(join(directory, 'JindiDrama.exe'))
}
await copyFile('docs/RELEASE-ROLLBACK.md', join(directory, 'RELEASE-ROLLBACK.md'))

const system = onMac ? 'macOS 13 Ventura 或更新版本（arm64 / x86_64）' : 'Windows 11 x64，需 Microsoft Edge WebView2 运行时'
const verification = onMac
  ? ['```sh', 'shasum -a 256 -c SHA256SUMS.txt', '```']
  : [
      '```powershell',
      "$manifest = Get-Content -LiteralPath SHA256SUMS.txt -ErrorAction Stop",
      "if (-not $manifest) { throw 'SHA-256 清单为空' }",
      '$manifest | ForEach-Object {',
      "  if ($_ -notmatch '^([0-9a-f]{64})  (.+)$') { throw '无效的 SHA-256 清单行' }",
      '  $expected = $Matches[1]',
      '  $name = $Matches[2]',
      "  if ($name -in @('.', '..') -or $name.IndexOfAny([IO.Path]::GetInvalidFileNameChars()) -ge 0) { throw '无效的文件名' }",
      "  if ((Get-FileHash -LiteralPath $name -Algorithm SHA256 -ErrorAction Stop).Hash -ne $expected) {",
      '    throw "SHA-256 校验失败 $name"',
      '  }',
      '  Write-Output "$name OK"',
      '}',
      '```',
    ]
const lines = [
  ...header,
  '- 系统要求 ' + system,
  '',
  '## 安装文件',
  '',
  ...files.map(([name, description]) => '- `' + name + '` ' + description),
  '- `PRIVACY.md` 隐私说明',
  '- `FRONTEND-LICENSES.md` 与 `GO-LICENSES.md` 第三方许可',
  '',
  '## 完整性校验',
  '',
  '将下载内容解压到同一目录，在该目录运行以下命令。使用与当前平台产物一起取得的清单。',
  '',
  ...verification,
  '',
  '## 功能与数据',
  '',
  '桌面版提供演示目录、授权本地片库、收藏、观看进度、数据备份和本地诊断。演示账号与演示目录可离线使用。红果正式服务待取得接入授权。',
  '',
  '可在设置中选择稳定版或预发布渠道。GitHub 提供匹配平台的安装包及 SHA-256 时，设置页可校验下载。也可打开发布页，安装由用户手动确认。旧版需要兼容副本时，先备份用户资料，再按回滚说明操作。',
  '',
  '[桌面发行回滚](RELEASE-ROLLBACK.md)',
  '',
]
await writeFile(join(directory, 'RELEASE-NOTES.md'), lines.join('\n'))
