import { constants } from 'node:fs'
import { copyFile, mkdir, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { hashFile, platformPackages, readVerifiedText, releaseLicenses, sharedReleaseDocuments, verify, verifyHostedPackage, writeManifest } from './package-checksums.mjs'
import { hostedPackageHeader } from './package-provenance.mjs'

const [windowsDirectory, macosDirectory, output, ...extra] = process.argv.slice(2)
const { PACKAGE_MODE: mode } = process.env

if (!windowsDirectory || !macosDirectory || !output || extra.length) {
  throw new Error('Usage: node scripts/assemble-release.mjs <windows-artifact> <macos-artifact> <output-directory>')
}
if (mode !== 'release') {
  throw new Error('Release assembly requires provenance from a GitHub Actions release-mode build')
}

const header = hostedPackageHeader(process.env)
const manifests = new Map()
const directories = new Map([['windows', windowsDirectory], ['macos', macosDirectory]])

async function verifiedText(platform, name) {
  return readVerifiedText(directories.get(platform), name, manifests.get(platform).get(name))
}

for (const [platform, directory] of directories) {
  manifests.set(platform, await verifyHostedPackage(directory, platform))
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
    '`SHA256SUMS.txt` 列出全部发行文件的 SHA-256。将全部文件下载到同一目录。macOS 在终端进入该目录后可运行 `shasum -a 256 -c SHA256SUMS.txt`。校验原始 ZIP 后再解压。单独下载一个文件时，可对照清单中同名文件的摘要。Windows 可用 PowerShell 的 `Get-FileHash -LiteralPath 路径 -Algorithm SHA256`。',
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
