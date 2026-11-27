import { execFileSync, spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { createReadStream, realpathSync } from 'node:fs'
import { readFile, readdir, stat, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { hostedPackageHeader, packageVersionPattern } from './package-provenance.mjs'

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
  const { checksums } = await verifyFiles(directory, platform)
  return checksums
}

async function verifyFiles(directory, platform) {
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

  const contents = await readFile(join(directory, manifest))
  const checksums = new Map()
  for (const line of contents.toString('utf8').replace(/\r?\n$/, '').split(/\r?\n/)) {
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
  const manifestFile = { digest: createHash('sha256').update(contents).digest('hex'), size: contents.length }
  return { checksums, manifestFile }
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

async function verifyReleaseTag(tag, directory) {
  if (!packageVersionPattern.test(tag)) throw new Error('Expected a valid release version')
  let revision
  try {
    revision = execFileSync('git', ['rev-parse', '--verify', '--end-of-options', 'refs/tags/' + tag + '^{commit}'],
      { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
  } catch (error) {
    throw new Error('Cannot resolve local Git tag ' + tag + ' to a commit', { cause: error })
  }
  const { checksums, manifestFile } = await verifyFiles(directory, 'release')
  const name = 'RELEASE-NOTES.md'
  const notes = await readVerifiedText(directory, name, checksums.get(name))
  const lines = notes.split(/\r?\n/, 4)
  const runLink = /\]\(https:\/\/github\.com\/([A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+)\/actions\/runs\/([1-9]\d*)\/attempts\/([1-9]\d*)\)$/.exec(lines[3])
  if (!runLink) throw new Error('Release notes are missing GitHub build provenance')
  const [, repository, run, attempt] = runLink
  const header = hostedPackageHeader({
    PACKAGE_MODE: 'release',
    PACKAGE_VERSION: tag,
    GITHUB_ACTIONS: 'true',
    GITHUB_REPOSITORY: repository,
    GITHUB_SHA: revision,
    GITHUB_RUN_ID: run,
    GITHUB_RUN_ATTEMPT: attempt,
  })
  if (header.some((line, index) => lines[index] !== line)) {
    throw new Error('Release handoff version or build commit does not match the local release tag')
  }
  console.log('Release tag ' + tag + ' matches build commit ' + revision)
  return { checksums, manifestFile, repository, revision, notes, run, attempt }
}

function remoteReleaseRevision(repository, tag) {
  const ref = 'refs/tags/' + tag
  const peeled = ref + '^{}'
  let output
  try {
    output = execFileSync('git', ['ls-remote', '--exit-code', '--tags',
      'https://github.com/' + repository + '.git', ref, peeled], {
      encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout: 20_000, maxBuffer: 64 * 1024,
      env: { ...process.env, GIT_TERMINAL_PROMPT: '0' },
    })
  } catch {
    throw new Error('Cannot resolve GitHub release tag ' + tag)
  }
  const refs = new Map()
  for (const line of output.trimEnd().split(/\r?\n/)) {
    const [sha, name, extra] = line.split('\t')
    if (name !== ref && name !== peeled) continue
    if (!/^[0-9a-f]{40}$/.test(sha) || extra !== undefined || refs.has(name)) {
      throw new Error('Unexpected GitHub release tag result: ' + tag)
    }
    refs.set(name, sha)
  }
  if (!refs.has(ref)) throw new Error('Cannot resolve exact GitHub release tag ' + tag)
  return refs.get(peeled) ?? refs.get(ref)
}

function normalizedReleaseBody(body) {
  return body.replace(/\r\n/g, '\n').trimEnd()
}

async function verifyReleaseJobs(jobsFile, { repository, revision, run, attempt }) {
  const response = JSON.parse(await readFile(jobsFile, 'utf8'))
  if (!Array.isArray(response?.jobs) || response.total_count !== response.jobs.length) {
    throw new Error('Expected the complete GitHub Actions jobs response for the release attempt')
  }
  const runURL = `https://api.github.com/repos/${repository}/actions/runs/${run}`.toLowerCase()
  const required = [
    'package-release (windows-latest)',
    'package-release (macos-latest)',
    'verify-windows-download',
    'verify-macos-intel',
    'assemble-release',
  ]
  for (const name of required) {
    const matching = response.jobs.filter((job) => job?.name === name)
    if (matching.length !== 1) throw new Error(`Expected one release job named ${name}`)
    const [job] = matching
    if (job.workflow_name !== 'package' || typeof job.run_url !== 'string' || job.run_url.toLowerCase() !== runURL ||
        String(job.run_id) !== run || String(job.run_attempt) !== attempt || job.head_sha !== revision ||
        job.status !== 'completed' || job.conclusion !== 'success') {
      throw new Error(`Release job ${name} must have succeeded for build ${run} attempt ${attempt} at ${revision}`)
    }
  }
  console.log(`GitHub Actions build ${run} attempt ${attempt} matches all ${required.length} successful release jobs`)
}

async function verifyReleaseArchive(tag, archiveFile, artifactFile, handoff, files) {
  const { repository, revision, run, attempt } = handoff
  const artifact = JSON.parse(await readFile(artifactFile, 'utf8'))
  const name = `release-handoff-${tag}-attempt-${attempt}`
  if (artifact?.name !== name || artifact.expired !== false ||
      !Number.isSafeInteger(artifact.id) || artifact.id < 1 ||
      !Number.isSafeInteger(artifact.size_in_bytes) || artifact.size_in_bytes < 1 ||
      typeof artifact.digest !== 'string' || !/^sha256:[0-9a-f]{64}$/.test(artifact.digest) ||
      typeof artifact.url !== 'string' ||
      artifact.url.toLowerCase() !== `https://api.github.com/repos/${repository.toLowerCase()}/actions/artifacts/${artifact.id}` ||
      String(artifact.workflow_run?.id) !== run || artifact.workflow_run?.head_sha !== revision) {
    throw new Error(`GitHub Actions artifact does not match ${name} from build ${run} at ${revision}`)
  }
  const expected = { size: artifact.size_in_bytes, digest: artifact.digest.slice(7), files }
  const helper = fileURLToPath(new URL('./verify-release-archive.py', import.meta.url))
  const python = process.platform === 'win32' ? 'python' : 'python3'
  const result = spawnSync(python, ['-I', helper, archiveFile], {
    encoding: 'utf8', input: JSON.stringify(expected), timeout: 120_000, windowsHide: true,
  })
  if (result.error) throw new Error('Cannot run the Python 3 release archive verifier', { cause: result.error })
  if (result.status !== 0) throw new Error(result.stderr.trim() || 'GitHub handoff ZIP verification failed')
  console.log(`GitHub Actions artifact ${artifact.id} matches the original ZIP and all ${Object.keys(files).length} handoff files`)
}

function matchesReleasePath(value, repository, expected) {
  const prefix = `https://github.com/${repository}/releases/`
  if (typeof value !== 'string' || !value.startsWith(prefix)) return false
  const segments = value.slice(prefix.length).split('/')
  try {
    return segments.length === expected.length &&
      expected.every((segment, index) => decodeURIComponent(segments[index]) === segment)
  } catch {
    return false
  }
}

function validPublicationTime(value) {
  if (typeof value !== 'string') return false
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.\d+)?Z$/.exec(value)
  if (!match) return false
  const date = new Date(match[1] + 'Z')
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 19) === match[1]
}

async function verifyGitHubRelease(draft, tag, directory, metadataFile, jobsFile, archiveFile, artifactFile) {
  const label = draft ? 'draft' : 'published release'
  const releaseLabel = draft ? 'GitHub Release draft' : 'GitHub published release'
  const assetLabel = draft ? 'Draft' : 'Published release'
  const handoff = await verifyReleaseTag(tag, directory)
  const { checksums, manifestFile, repository, revision, notes } = handoff
  const release = JSON.parse(await readFile(metadataFile, 'utf8'))
  const prerelease = tag.split('+', 1)[0].includes('-')
  const api = `https://api.github.com/repos/${repository}`.toLowerCase()
  if (release?.draft !== draft || release.tag_name !== tag || release.prerelease !== prerelease ||
      !Number.isSafeInteger(release.id) || release.id < 1 ||
      typeof release.url !== 'string' || release.url.toLowerCase() !== `${api}/releases/${release.id}`) {
    throw new Error(`${releaseLabel} does not match the repository, tag, and release channel`)
  }
  if (!draft && (!validPublicationTime(release.published_at) ||
      !matchesReleasePath(release.html_url, repository, ['tag', tag]))) {
    throw new Error('Published release is missing a publication time or has an unexpected public release page')
  }
  if (typeof release.body !== 'string' || normalizedReleaseBody(release.body) !== normalizedReleaseBody(notes)) {
    throw new Error(`${releaseLabel} body does not match RELEASE-NOTES.md`)
  }
  if (!Array.isArray(release.assets)) throw new Error(`Missing ${releaseLabel} assets`)

  checksums.set(manifest, manifestFile.digest)
  const files = new Map()
  const assetIDs = new Set()
  for (const asset of release.assets) {
    const name = asset?.name
    if (!checksums.has(name) || files.has(name)) {
      throw new Error(`Unexpected or duplicate ${label} asset: ${JSON.stringify(name)}`)
    }
    const { size } = name === manifest ? manifestFile : await stat(join(directory, name))
    if (!Number.isSafeInteger(asset.id) || asset.id < 1 || assetIDs.has(asset.id) || asset.state !== 'uploaded' ||
        size !== asset.size || asset.digest !== 'sha256:' + checksums.get(name)) {
      throw new Error(`${assetLabel} asset ID, upload state, size or SHA-256 mismatch: ${name}`)
    }
    if (!draft && (typeof asset.url !== 'string' || asset.url.toLowerCase() !== `${api}/releases/assets/${asset.id}` ||
        !matchesReleasePath(asset.browser_download_url, repository, ['download', tag, name]))) {
      throw new Error(`Published release download URL mismatch: ${name}`)
    }
    assetIDs.add(asset.id)
    files.set(name, { size, digest: checksums.get(name) })
  }
  for (const name of checksums.keys()) {
    if (!files.has(name)) throw new Error(`Missing ${label} asset: ${name}`)
  }
  await verifyReleaseJobs(jobsFile, handoff)
  await verifyReleaseArchive(tag, archiveFile, artifactFile, handoff, Object.fromEntries(files))
  if (remoteReleaseRevision(repository, tag) !== revision) {
    throw new Error('GitHub release tag does not match the handoff build commit')
  }
  console.log('GitHub release tag ' + tag + ' matches build commit ' + revision)
  console.log(`${releaseLabel} ${release.id} matches all ${files.size} handoff files`)
}

function isMainModule(entry) {
  // Older Node 22 versions lack import.meta.main. Eval uses argv[1] for the first user argument.
  if (!entry || entry === '-' || process.execArgv.some((arg) => /^(-[ep]+$|--(eval|print)(=|$))/.test(arg))) return false
  if (import.meta.url === pathToFileURL(entry).href) return true
  try {
    return realpathSync(entry) === realpathSync(fileURLToPath(import.meta.url))
  } catch {
    return false
  }
}

if (import.meta.main ?? isMainModule(process.argv[1])) {
  const [first, platform, directory, ...extra] = process.argv.slice(2)
  if (first === '--verify' && (platformPackages.has(platform) || platform === 'release') && directory && extra.length === 0) {
    await verify(directory, platform)
  } else if (first === '--verify-hosted' && platformPackages.has(platform) && directory && extra.length === 0) {
    await verifyHostedPackage(directory, platform)
  } else if (first === '--verify-release-tag' && platform && directory && extra.length === 0) {
    await verifyReleaseTag(platform, directory)
  } else if ((first === '--verify-release-draft' || first === '--verify-release-published') && platform && directory && extra.length === 4) {
    await verifyGitHubRelease(first === '--verify-release-draft', platform, directory, ...extra)
  } else if (first && !first.startsWith('--') && platform === undefined) {
    await generate(first)
  } else {
    throw new Error('Usage: node scripts/package-checksums.mjs <package-directory> | --verify <windows|macos|release> <download-directory> | --verify-hosted <windows|macos> <download-directory> | --verify-release-tag <tag> <release-directory> | <--verify-release-draft|--verify-release-published> <tag> <release-directory> <github-release.json> <github-jobs.json> <original-handoff.zip> <github-artifact.json>')
  }
}
