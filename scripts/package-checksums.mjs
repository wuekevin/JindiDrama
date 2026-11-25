import { createHash } from 'node:crypto'
import { createReadStream } from 'node:fs'
import { readFile, readdir, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { hostedPackageHeader } from './package-provenance.mjs'

const manifest = 'SHA256SUMS.txt'
const sidecars = ['RELEASE-NOTES.md', 'RELEASE-ROLLBACK.md', 'PRIVACY.md', 'FRONTEND-LICENSES.md', 'GO-LICENSES.md']
export const platformPackages = new Map([
  ['windows', ['JindiDrama-amd64-installer.exe', 'JindiDrama.exe']],
  ['macos', ['JindiDrama-macos-universal.zip']],
])
export const sharedReleaseDocuments = ['PRIVACY.md', 'RELEASE-ROLLBACK.md']
const perPlatformLicenses = ['FRONTEND-LICENSES', 'GO-LICENSES']
export const releaseLicenses = (platform) => perPlatformLicenses.map((name) => [name + '.md', name + '-' + platform + '.md'])
const releaseFiles = [
  ...[...platformPackages.values()].flat(),
  ...sharedReleaseDocuments,
  ...['windows', 'macos'].flatMap((platform) => releaseLicenses(platform).map(([, target]) => target)),
  'RELEASE-NOTES.md',
]

export async function hashFile(path) {
  const hash = createHash('sha256')
  for await (const chunk of createReadStream(path)) hash.update(chunk)
  return hash.digest('hex')
}

async function generate(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  // Match the upload step's visible files and Windows glob casing.
  const packageExtension = process.platform === 'win32' ? /\.(exe|zip)$/i : /\.(exe|zip)$/
  const files = entries.filter((entry) => entry.isFile()).map((entry) => entry.name)
  const packages = files.filter((name) => !name.startsWith('.') && packageExtension.test(name))
  if (packages.length === 0) throw new Error(`No .exe or .zip packages found in ${directory}`)

  const notices = files.filter((name) => sidecars.includes(name))
  const checksums = new Map()
  for (const name of [...packages, ...notices]) {
    checksums.set(name, await hashFile(join(directory, name)))
  }
  await writeManifest(directory, checksums)
}

export async function writeManifest(directory, checksums) {
  const lines = [...checksums.keys()].sort().map((name) => `${checksums.get(name)}  ${name}`)
  await writeFile(join(directory, manifest), lines.join('\n') + '\n')
}

export async function verify(directory, platform) {
  const expected = new Set(platform === 'release' ? releaseFiles : [...platformPackages.get(platform), ...sidecars])
  const entries = await readdir(directory, { withFileTypes: true })
  const files = new Set()
  for (const entry of entries) {
    if (!entry.isFile() || (entry.name !== manifest && !expected.has(entry.name))) {
      throw new Error(`Unexpected artifact entry: ${JSON.stringify(entry.name)}`)
    }
    files.add(entry.name)
  }
  for (const name of [...expected, manifest]) {
    if (!files.has(name)) throw new Error(`Missing artifact file: ${name}`)
  }

  const contents = await readFile(join(directory, manifest), 'utf8')
  const checksums = new Map()
  for (const line of contents.replace(/\r?\n$/, '').split(/\r?\n/)) {
    const match = /^([a-fA-F0-9]{64})  (.+)$/.exec(line)
    if (!match) throw new Error(`Malformed ${manifest} entry`)
    const [, digest, name] = match
    if (!expected.has(name) || checksums.has(name)) {
      throw new Error(`Unexpected or duplicate checksum entry: ${JSON.stringify(name)}`)
    }
    checksums.set(name, digest.toLowerCase())
  }
  for (const name of expected) {
    if (!checksums.has(name)) throw new Error(`Missing checksum for ${name}`)
  }
  for (const [name, digest] of checksums) {
    if (await hashFile(join(directory, name)) !== digest) throw new Error(`SHA-256 mismatch: ${name}`)
    console.log(`${name}: OK`)
  }
  return checksums
}

export async function readVerifiedText(directory, name, digest) {
  const contents = await readFile(join(directory, name))
  if (createHash('sha256').update(contents).digest('hex') !== digest) {
    throw new Error(`SHA-256 mismatch: ${name}`)
  }
  return contents.toString('utf8')
}

export async function verifyHostedPackage(directory, platform, env = process.env) {
  const header = hostedPackageHeader(env)
  const checksums = await verify(directory, platform)
  const name = 'RELEASE-NOTES.md'
  const notes = await readVerifiedText(directory, name, checksums.get(name))
  const lines = notes.split(/\r?\n/, header.length)
  if (header.some((line, index) => lines[index] !== line)) {
    throw new Error(`${platform} release notes do not match the expected mode, version, commit, run, and attempt`)
  }
  return checksums
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [first, platform, directory, ...extra] = process.argv.slice(2)
  if (first === '--verify' && (platformPackages.has(platform) || platform === 'release') && directory && extra.length === 0) {
    await verify(directory, platform)
  } else if (first === '--verify-hosted' && platformPackages.has(platform) && directory && extra.length === 0) {
    await verifyHostedPackage(directory, platform)
  } else if (first && !first.startsWith('--') && platform === undefined) {
    await generate(first)
  } else {
    throw new Error('Usage: node scripts/package-checksums.mjs <package-directory> | --verify <windows|macos|release> <download-directory> | --verify-hosted <windows|macos> <download-directory>')
  }
}
