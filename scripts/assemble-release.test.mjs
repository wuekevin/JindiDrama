import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { mkdir, mkdtemp, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { fileURLToPath, pathToFileURL } from 'node:url'

const checksums = fileURLToPath(new URL('./package-checksums.mjs', import.meta.url))
const assemble = fileURLToPath(new URL('./assemble-release.mjs', import.meta.url))
const repository = 'wuekevin/JindiDrama'
const revision = 'a'.repeat(40)
const version = 'v0.1.0-rc.1'
const runID = '12345'
const attempt = '2'
const env = {
  ...process.env, PACKAGE_VERSION: version, PACKAGE_MODE: 'release',
  GITHUB_ACTIONS: 'true', GITHUB_REPOSITORY: repository, GITHUB_SHA: revision, GITHUB_RUN_ID: runID, GITHUB_RUN_ATTEMPT: attempt,
}
const releaseNotes = [
  '# 金帝短剧 ' + version,
  '',
  '- 打包类型 正式候选',
  '- 构建来源 [提交 `' + revision + '`](https://github.com/' + repository + '/commit/' + revision +
    ') · [打包运行 ' + runID + '，第 ' + attempt + ' 次](https://github.com/' + repository +
    '/actions/runs/' + runID + '/attempts/' + attempt + ')',
  '',
].join('\n')

function run(script, args, options = {}) {
  const result = spawnSync(process.execPath, [script, ...args], { encoding: 'utf8', ...options })
  if (result.error) throw result.error
  return result
}

function generate(directory) {
  const result = run(checksums, [directory])
  assert.equal(result.status, 0, result.stderr)
}

async function fixture(t, notes = releaseNotes) {
  const root = await mkdtemp(join(tmpdir(), 'jindi-release-handoff-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  const windows = join(root, 'windows')
  const mac = join(root, 'macos')
  const output = join(root, 'release')
  for (const [platform, dir] of [['windows', windows], ['macos', mac]]) {
    await mkdir(dir)
    await writeFile(join(dir, 'RELEASE-NOTES.md'), notes)
    await writeFile(join(dir, 'RELEASE-ROLLBACK.md'), 'rollback\n')
    await writeFile(join(dir, 'PRIVACY.md'), platform === 'windows' ? 'privacy\r\n' : 'privacy\n')
    await writeFile(join(dir, 'FRONTEND-LICENSES.md'), 'frontend licenses ' + platform)
    await writeFile(join(dir, 'GO-LICENSES.md'), 'go licenses ' + platform)
  }
  await writeFile(join(windows, 'JindiDrama.exe'), 'portable')
  await writeFile(join(windows, 'JindiDrama-amd64-installer.exe'), 'installer')
  await writeFile(join(mac, 'JindiDrama-macos-universal.zip'), 'universal')
  generate(windows)
  generate(mac)
  return { windows, mac, output, args: [windows, mac, output] }
}

function git(cwd, ...args) {
  const result = spawnSync('git', args, { cwd, encoding: 'utf8' })
  if (result.error) throw result.error
  assert.equal(result.status, 0, result.stderr)
  return result.stdout.trim()
}

test('verifies the handoff against GitHub release jobs, tags, draft notes and uploaded assets', async (t) => {
  const cwd = await mkdtemp(join(tmpdir(), 'jindi-release-tag-'))
  t.after(() => rm(cwd, { recursive: true, force: true }))
  git(cwd, 'init', '--quiet', '--object-format=sha1')
  const commit = (message) => git(cwd, '-c', 'user.name=JindiDrama',
    '-c', 'user.email=test@example.com', '-c', 'commit.gpgsign=false', 'commit', '--quiet',
    '--allow-empty', '-m', message)
  commit('packaged source')
  const sha = git(cwd, 'rev-parse', 'HEAD')
  git(cwd, 'branch', version)
  const { output, args } = await fixture(t, releaseNotes.replaceAll(revision, sha))
  const assembled = run(assemble, args, { env: { ...env, GITHUB_SHA: sha } })
  assert.equal(assembled.status, 0, assembled.stderr)

  const check = (tag = version) => run(checksums, ['--verify-release-tag', tag, output], { cwd })
  assert.match(check().stderr, /Cannot resolve local Git tag/)
  assert.match(check('--not-a-tag').stderr, /Expected a valid release version/)
  git(cwd, 'tag', version)
  const lightweight = check()
  assert.equal(lightweight.status, 0, lightweight.stderr)
  assert.match(lightweight.stdout, new RegExp(sha))
  git(cwd, 'tag', 'v0.1.0-rc.2')
  assert.match(check('v0.1.0-rc.2').stderr, /does not match the local release tag/)
  commit('wrong source')
  const wrongSHA = git(cwd, 'rev-parse', 'HEAD')
  git(cwd, 'tag', '--force', version)
  assert.match(check().stderr, /does not match the local release tag/)

  git(cwd, '-c', 'user.name=JindiDrama', '-c', 'user.email=test@example.com',
    '-c', 'tag.gpgSign=false', 'tag', '--force', '-a', version, sha, '-m', 'candidate')
  const annotated = check()
  assert.equal(annotated.status, 0, annotated.stderr)
  assert.match(annotated.stdout, new RegExp(sha))

  const assets = await Promise.all((await readdir(output)).map(async (name, index) => {
    const bytes = await readFile(join(output, name))
    return { id: index + 101, name, state: 'uploaded', size: bytes.length, digest: 'sha256:' + createHash('sha256').update(bytes).digest('hex') }
  }))
  const remote = join(cwd, 'remote.git')
  git(cwd, 'init', '--bare', '--quiet', '--object-format=sha1', remote)
  const remoteURL = 'https://github.com/' + repository + '.git'
  git(cwd, 'config', 'url.' + pathToFileURL(remote).href + '.insteadOf', remoteURL)
  git(cwd, 'push', '--quiet', remote, 'refs/tags/' + version, 'HEAD:refs/heads/' + version)

  const body = await readFile(join(output, 'RELEASE-NOTES.md'), 'utf8')
  const draft = {
    id: 123, url: `https://api.github.com/repos/${repository}/releases/123`,
    tag_name: version, draft: true, prerelease: true, assets, body,
  }
  const draftFile = join(cwd, 'draft.json')
  const jobsFile = join(cwd, 'release-jobs.json')
  const artifactFile = join(cwd, 'github-artifact.json')
  const archiveFile = join(cwd, 'release-handoff.zip')
  const python = process.platform === 'win32' ? 'python' : 'python3'
  const entries = await readdir(output)
  async function createArchive(names = entries) {
    const result = spawnSync(python, ['-I', '-m', 'zipfile', '-c', archiveFile, ...names], {
      cwd: output, encoding: 'utf8',
    })
    assert.equal(result.status, 0, result.stderr || result.error?.message)
    return readFile(archiveFile)
  }
  const archive = await createArchive()
  const zipMetadata = (bytes) => ({
    digest: 'sha256:' + createHash('sha256').update(bytes).digest('hex'), size_in_bytes: bytes.length,
  })
  const artifact = {
    id: 321, url: 'https://api.github.com/repos/' + repository + '/actions/artifacts/321',
    name: `release-handoff-${version}-attempt-${attempt}`, expired: false,
    workflow_run: { id: Number(runID), head_sha: sha }, ...zipMetadata(archive),
  }
  const jobs = [
    'package-release (windows-latest)', 'package-release (macos-latest)',
    'verify-windows-download', 'verify-macos-intel', 'assemble-release',
  ].map((name) => ({
    name, workflow_name: 'package', run_id: Number(runID), run_attempt: Number(attempt), head_sha: sha,
    run_url: 'https://api.github.com/repos/' + repository + '/actions/runs/' + runID,
    status: 'completed', conclusion: 'success',
  }))
  async function checkDraft(data, { releaseJobs = jobs, total = releaseJobs.length, releaseArtifact = artifact } = {}) {
    await writeFile(draftFile, JSON.stringify(data))
    await writeFile(jobsFile, JSON.stringify({ total_count: total, jobs: releaseJobs }))
    await writeFile(artifactFile, JSON.stringify(releaseArtifact))
    return run(checksums, ['--verify-release-draft', version, output, draftFile, jobsFile, archiveFile, artifactFile], { cwd })
  }
  const validDraft = await checkDraft(draft)
  assert.equal(validDraft.status, 0, validDraft.stderr)
  assert.match(validDraft.stdout, /matches all 11 handoff files/)
  assert.match(validDraft.stdout, /build 12345 attempt 2 matches all 5 successful release jobs/)
  assert.match(validDraft.stdout, /artifact 321 matches the original ZIP and all 11 handoff files/)
  for (const wrong of [
    { name: artifact.name.replace('attempt-2', 'attempt-1') }, { expired: true },
    { url: 'https://api.github.com/repos/example/Other/actions/artifacts/321' },
    { workflow_run: { id: 67890, head_sha: sha } }, { workflow_run: { id: Number(runID), head_sha: wrongSHA } },
  ]) {
    const result = await checkDraft(draft, { releaseArtifact: { ...artifact, ...wrong } })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /GitHub Actions artifact does not match/)
  }
  await writeFile(archiveFile, archive.subarray(1))
  assert.match((await checkDraft(draft)).stderr, /ZIP size mismatch/)
  const damaged = Buffer.from(archive)
  damaged[0] ^= 1
  await writeFile(archiveFile, damaged)
  assert.match((await checkDraft(draft)).stderr, /ZIP SHA-256 mismatch/)

  // Updating the outer digest must not let a different binary through.
  const portable = join(output, 'JindiDrama.exe')
  const originalPortable = await readFile(portable)
  await writeFile(portable, Buffer.alloc(originalPortable.length, 0x61))
  const different = await createArchive()
  await writeFile(portable, originalPortable)
  const replaced = await checkDraft(draft, { releaseArtifact: { ...artifact, ...zipMetadata(different) } })
  assert.match(replaced.stderr, /ZIP entry SHA-256 mismatch: JindiDrama.exe/)
  const incomplete = await createArchive(entries.filter((name) => name !== 'SHA256SUMS.txt'))
  assert.match((await checkDraft(draft, { releaseArtifact: { ...artifact, ...zipMetadata(incomplete) } })).stderr,
    /ZIP has missing, duplicate or unexpected entries/)
  await writeFile(archiveFile, archive)
  assert.match((await checkDraft(draft, { total: jobs.length + 1 })).stderr, /complete GitHub Actions jobs response/)
  assert.match((await checkDraft(draft, { releaseJobs: jobs.slice(1) })).stderr, /Expected one release job named package-release/)
  assert.match((await checkDraft(draft, { releaseJobs: [...jobs, jobs.at(-1)] })).stderr, /Expected one release job named assemble-release/)
  for (const wrong of [
    { run_attempt: 1 }, { run_id: 67890 }, { head_sha: wrongSHA }, { workflow_name: 'CI' },
    { run_url: 'https://api.github.com/repos/example/Other/actions/runs/' + runID },
    { status: 'in_progress' }, { conclusion: 'failure' }, { conclusion: 'skipped' },
  ]) {
    const rejected = await checkDraft(draft, { releaseJobs: [...jobs.slice(0, -1), { ...jobs.at(-1), ...wrong }] })
    assert.equal(rejected.status, 1)
    assert.match(rejected.stderr, /Release job assemble-release must have succeeded/)
  }
  assert.match(validDraft.stdout, new RegExp('GitHub release tag .* matches build commit ' + sha))
  const normalizedDraft = await checkDraft({ ...draft, body: body.trimEnd().replace(/\n/g, '\r\n') })
  assert.equal(normalizedDraft.status, 0, normalizedDraft.stderr)
  for (const invalidBody of [null, body + '\nUnverified change']) {
    const rejected = await checkDraft({ ...draft, body: invalidBody })
    assert.equal(rejected.status, 1)
    assert.match(rejected.stderr, /draft body does not match RELEASE-NOTES.md/)
  }

  git(remote, 'update-ref', 'refs/tags/' + version, wrongSHA)
  const moved = await checkDraft(draft)
  assert.equal(moved.status, 1)
  assert.match(moved.stderr, /GitHub release tag does not match the handoff build commit/)
  git(remote, 'update-ref', '-d', 'refs/tags/' + version)
  assert.match((await checkDraft(draft)).stderr, /Cannot resolve GitHub release tag/)
  git(remote, 'update-ref', 'refs/tags/' + version, sha)
  const remoteLightweight = await checkDraft(draft)
  assert.equal(remoteLightweight.status, 0, remoteLightweight.stderr)

  for (const wrong of [{ draft: false }, { prerelease: false }, { tag_name: 'v0.2.0' },
    { url: 'https://api.github.com/repos/example/Other/releases/123' }]) {
    const result = await checkDraft({ ...draft, ...wrong })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /does not match the repository, tag, and release channel/)
  }
  const withoutManifest = await checkDraft({ ...draft, assets: assets.filter(({ name }) => name !== 'SHA256SUMS.txt') })
  assert.match(withoutManifest.stderr, /Missing draft asset: SHA256SUMS.txt/)
  const duplicate = await checkDraft({ ...draft, assets: [...assets, assets[0]] })
  assert.match(duplicate.stderr, /Unexpected or duplicate draft asset/)
  for (const wrong of [{ id: 0 }, { state: 'starter' }, { size: 0 }, { digest: null }, { digest: 'sha256:' + '0'.repeat(64) }]) {
    const result = await checkDraft({ ...draft, assets: assets.map((asset) =>
      asset.name === 'JindiDrama-amd64-installer.exe' ? { ...asset, ...wrong } : asset) })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /mismatch: JindiDrama-amd64-installer.exe/)
  }
  for (const name of ['JindiDrama-macos-universal.zip', 'SHA256SUMS.txt']) {
    const result = await checkDraft({ ...draft, assets: assets.map((asset) =>
      asset.name === name ? { ...asset, digest: 'sha256:' + '0'.repeat(64) } : asset) })
    assert.equal(result.status, 1)
    assert.ok(result.stderr.includes('mismatch: ' + name), result.stderr)
  }
  await writeFile(join(output, 'JindiDrama.exe'), 'changed')
  assert.match(check().stderr, /SHA-256 mismatch: JindiDrama.exe/)
})

test('assembles both verified platforms and emits one complete release manifest', async (t) => {
  const { output, args } = await fixture(t)
  const result = run(assemble, args, { env })
  assert.equal(result.status, 0, result.stderr)
  const check = run(checksums, ['--verify', 'release', output])
  assert.equal(check.status, 0, check.stderr)
  assert.equal(check.stdout.trim().split('\n').length, 10)
  assert.equal(await readFile(join(output, 'GO-LICENSES-windows.md'), 'utf8'), 'go licenses windows')
  assert.equal(await readFile(join(output, 'GO-LICENSES-macos.md'), 'utf8'), 'go licenses macos')
  const notes = await readFile(join(output, 'RELEASE-NOTES.md'), 'utf8')
  assert.match(notes, /打包运行 12345/)
  assert.match(notes, /终端进入该目录后可运行.*校验原始 ZIP 后再解压/)
  await writeFile(join(output, 'JindiDrama.exe'), 'changed')
  assert.match(run(checksums, ['--verify', 'release', output]).stderr, /SHA-256 mismatch: JindiDrama.exe/)
})

test('rejects smoke, mixed-version, mixed-commit, mixed-run, and mixed-attempt inputs', async (t) => {
  const { windows, output, args } = await fixture(t)
  for (const notes of [
    releaseNotes.replace('正式候选', '测试包'),
    releaseNotes.replace(version, 'v0.1.1-rc.1'),
    releaseNotes.replaceAll(revision, 'b'.repeat(40)),
    releaseNotes.replaceAll(runID, '54321'),
    releaseNotes.replace('第 2 次', '第 1 次').replace('/attempts/2)', '/attempts/1)'),
  ]) {
    await writeFile(join(windows, 'RELEASE-NOTES.md'), notes)
    generate(windows)
    const result = run(assemble, args, { env })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /release notes do not match/)
    await assert.rejects(stat(output), { code: 'ENOENT' })
  }
})

test('rejects altered source binaries and incompatible shared disclosures', async (t) => {
  const { mac, output, args } = await fixture(t)
  await writeFile(join(mac, 'JindiDrama-macos-universal.zip'), 'changed')
  assert.match(run(assemble, args, { env }).stderr, /SHA-256 mismatch/)
  await writeFile(join(mac, 'PRIVACY.md'), 'conflicting policy\n')
  generate(mac)
  assert.match(run(assemble, args, { env }).stderr, /different PRIVACY.md content/)
  await assert.rejects(stat(output), { code: 'ENOENT' })
})

test('requires release mode and preserves an existing handoff directory', async (t) => {
  const { output, args } = await fixture(t)
  const smoke = run(assemble, args, { env: { ...env, PACKAGE_MODE: 'smoke' } })
  assert.equal(smoke.status, 1)
  assert.match(smoke.stderr, /requires provenance/)
  await mkdir(output)
  await writeFile(join(output, 'existing'), 'keep')
  const exists = run(assemble, args, { env })
  assert.equal(exists.status, 1)
  assert.match(exists.stderr, /EEXIST/)
  assert.equal(await readFile(join(output, 'existing'), 'utf8'), 'keep')
})

test('hosted verification matches the current smoke or release attempt on both platforms', async (t) => {
  const { windows, mac } = await fixture(t)
  for (const [platform, directory] of [['windows', windows], ['macos', mac]]) {
    const args = ['--verify-hosted', platform, directory]
    const release = run(checksums, args, { env })
    assert.equal(release.status, 0, release.stderr)

    await writeFile(join(directory, 'RELEASE-NOTES.md'), releaseNotes.replace('正式候选', '测试包'))
    generate(directory)
    const smokeEnv = { ...env, PACKAGE_MODE: 'smoke' }
    const smoke = run(checksums, args, { env: smokeEnv })
    assert.equal(smoke.status, 0, smoke.stderr)

    for (const change of [
      { PACKAGE_MODE: 'release' },
      { PACKAGE_VERSION: 'v0.1.1-rc.1' },
      { GITHUB_SHA: 'b'.repeat(40) },
      { GITHUB_REPOSITORY: 'other/JindiDrama' },
      { GITHUB_RUN_ID: '54321' },
      { GITHUB_RUN_ATTEMPT: '3' },
    ]) {
      const wrong = run(checksums, args, { env: { ...smokeEnv, ...change } })
      assert.equal(wrong.status, 1)
      assert.match(wrong.stderr, /release notes do not match/)
    }
  }
})
