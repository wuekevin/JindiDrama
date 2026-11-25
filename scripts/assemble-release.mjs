import { createHash } from 'node:crypto'
import { constants } from 'node:fs'
import { copyFile, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { hashFile, platformPackages, releaseLicenses, sharedReleaseDocuments, verify, writeManifest } from './package-checksums.mjs'
import { githubBuildSource } from './package-provenance.mjs'

const [windowsDirectory, macosDirectory, output, ...extra] = process.argv.slice(2)
const { PACKAGE_VERSION: version, PACKAGE_MODE: mode, GITHUB_ACTIONS: inActions } = process.env

if (!windowsDirectory || !macosDirectory || !output || extra.length) {
  throw new Error('Usage: node scripts/assemble-release.mjs <windows-artifact> <macos-artifact> <output-directory>')
}
if (inActions !== 'true' || mode !== 'release' ||
    !/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(version ?? '')) {
  throw new Error('Release assembly requires provenance from a GitHub Actions release-mode build')
}

const source = '- 构建来源 ' + githubBuildSource(process.env)
const header = ['# 金帝短剧 ' + version, '', '- 打包类型 正式候选', source]
const manifests = new Map()
const directories = new Map([['windows', windowsDirectory], ['macos', macosDirectory]])

async function verifiedText(platform, name) {
  const contents = await readFile(join(directories.get(platform), name))
  if (createHash('sha256').update(contents).digest('hex') !== manifests.get(platform).get(name)) {
    throw new Error(`SHA-256 mismatch: ${platform}/${name}`)
  }
  return contents.toString('utf8')
}

for (const [platform, directory] of directories) {
  manifests.set(platform, await verify(directory, platform))
  const notes = await verifiedText(platform, 'RELEASE-NOTES.md')
  const [title, blank, kind, provenance] = notes.split(/\r?\n/, header.length)
  if ([title, blank, kind, provenance].some((line, index) => line !== header[index])) {
    throw new Error(`${platform} release notes do not match the expected mode, version, commit, run, and attempt`)
  }
}

for (const name of sharedReleaseDocuments) {
  const windows = await verifiedText('windows', name)
  const macos = await verifiedText('macos', name)
  if (windows.replaceAll('\r\n', '\n') !== macos.replaceAll('\r\n', '\n')) {
    throw new Error(`The platform artifacts have different ${name} content`)
  }
}

const manifest = new Map()
async function copyVerified(platform, original, target = original) {
  await copyFile(join(directories.get(platform), original), join(output, target), constants.COPYFILE_EXCL)
  manifest.set(target, manifests.get(platform).get(original))
}

// Exclusive creation preserves any existing output or downloaded artifact.
await mkdir(output, { mode: 0o700 })
try {
  for (const [platform, names] of platformPackages) {
    for (const name of names) await copyVerified(platform, name)
    for (const [original, target] of releaseLicenses(platform)) await copyVerified(platform, original, target)
  }
  for (const name of sharedReleaseDocuments) await copyVerified('macos', name)
  const notesName = 'RELEASE-NOTES.md'
  const notes = [
    ...header,
    '',
    '## 下载',
    '',
    '- Windows 11 x64（需 Microsoft Edge WebView2 运行时）。`JindiDrama-amd64-installer.exe` 为安装包，`JindiDrama.exe` 为便携版。',
    '- `JindiDrama-macos-universal.zip` 支持 macOS 13 或更新版本，适用于 Apple Silicon 和 Intel。',
    '- `PRIVACY.md` 为隐私说明，`RELEASE-ROLLBACK.md` 为数据回滚说明。随附许可文件按平台命名。',
    '',
    '## 文件校验',
    '',
    '`SHA256SUMS.txt` 列出全部发行文件的 SHA-256。下载全部文件到同一目录后，macOS 可运行 `shasum -a 256 -c SHA256SUMS.txt`。单独下载一个文件时，可对照清单中同名文件的摘要。Windows 可用 PowerShell 的 `Get-FileHash -LiteralPath 路径 -Algorithm SHA256`。',
    '',
    '## 功能与数据',
    '',
    '桌面应用支持离线演示目录、授权本地片库、收藏、观看进度、数据备份和本地诊断。红果正式服务待取得接入授权。检测到含 SHA-256 的新版安装包时，桌面设置支持校验下载，由用户手动安装。',
    '',
    '退回旧版前先备份用户资料。数据库版本不兼容时，请下载 `RELEASE-ROLLBACK.md` 并按说明生成独立的兼容副本。',
    '',
  ]
  const notesPath = join(output, notesName)
  await writeFile(notesPath, notes.join('\n'), { flag: 'wx' })
  manifest.set(notesName, await hashFile(notesPath))
  await writeManifest(output, manifest)
  await verify(output, 'release')
} catch (error) {
  await rm(output, { recursive: true, force: true })
  throw error
}
