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

test('verifies the handoff against GitHub release jobs, tags, draft and published metadata, and uploaded assets', async (t) => {
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
  const tagResponsesFile = join(cwd, 'github-tag-responses.json')
  const githubFetchFixture = join(cwd, 'github-fetch-fixture.mjs')
  const api = 'https://api.github.com/repos/' + repository
  const ref = 'refs/tags/' + version
  const refURL = api + '/git/ref/tags/' + encodeURIComponent(version)
  const annotationSHA = git(cwd, 'rev-parse', ref)
  const annotationURL = api + '/git/tags/' + annotationSHA
  const annotatedReference = { ref, object: { type: 'tag', sha: annotationSHA } }
  const commitObject = { type: 'commit', sha }
  const lightweightReference = { ref, object: commitObject }
  const annotatedResponses = {
    [refURL]: { body: annotatedReference },
    [annotationURL]: { body: { sha: annotationSHA, object: commitObject } },
  }
  const assetPaths = Object.fromEntries(assets.map(({ id, name }) => [api + '/releases/assets/' + id, join(output, name)]))
  await writeFile(githubFetchFixture, `
    import { readFile } from 'node:fs/promises'
    const paths = ${JSON.stringify(assetPaths)}
    const tagResponses = JSON.parse(await readFile(${JSON.stringify(tagResponsesFile)}, 'utf8'))
    globalThis.fetch = async (url, { headers, redirect, credentials }) => {
      const address = String(url)
      const path = paths[address]
      if (path && headers.Accept === 'application/octet-stream') {
        const bytes = await readFile(path)
        if (process.env.JINDI_CORRUPT_PUBLIC && path.endsWith('SHA256SUMS.txt')) bytes[0] ^= 1
        return new Response(bytes, { headers: { 'Content-Length': bytes.length } })
      }
      if (!Object.hasOwn(tagResponses, address) || headers.Accept !== 'application/vnd.github+json' ||
          redirect !== 'error' || credentials !== 'omit') throw new Error('unexpected GitHub request')
      const { body, status = 200 } = tagResponses[address]
      return new Response(body === undefined ? null : JSON.stringify(body), { status })
    }
  `)
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
    ['package-release (windows-latest)', [
      'verify distribution source history',
      'verify frontend release regressions',
      'verify native Go release regressions',
      'build (windows, NSIS installer)',
      'confirm dependency files stayed unchanged',
      'sign application and installer (windows release)',
      'verify installer and WebView2 startup (windows)',
      'generate SHA-256 checksums',
    ]],
    ['package-release (macos-latest)', [
      'verify distribution source history',
      'verify frontend release regressions',
      'verify native Go release regressions',
      'build and verify deployment target (macos)',
      'confirm dependency files stayed unchanged',
      'sign application (macos release)',
      'notarize application (macos release)',
      'zip (macos)',
      'verify archived desktop startup (macos)',
      'generate SHA-256 checksums',
    ]],
    ['verify-windows-download', ['verify downloaded Windows packages']],
    ['verify-macos-intel', ['verify the downloaded Universal archive on Intel']],
    ['assemble-release', ['assemble and verify the release handoff']],
  ].map(([name, names]) => ({
    name, workflow_name: 'package', run_id: Number(runID), run_attempt: Number(attempt), head_sha: sha,
    run_url: 'https://api.github.com/repos/' + repository + '/actions/runs/' + runID,
    status: 'completed', conclusion: 'success',
    steps: names.map((name, index) => ({ name, status: 'completed', conclusion: 'success', number: index * 2 + 5 })),
  }))
  async function checkRelease(data, {
    releaseJobs = jobs, total = releaseJobs.length, releaseArtifact = artifact, published = false,
    downloadPublic = false, corruptPublic = false, tagResponses = annotatedResponses,
  } = {}) {
    await writeFile(draftFile, JSON.stringify(data))
    await writeFile(jobsFile, JSON.stringify({ total_count: total, jobs: releaseJobs }))
    await writeFile(artifactFile, JSON.stringify(releaseArtifact))
    await writeFile(tagResponsesFile, JSON.stringify(tagResponses))
    const option = downloadPublic ? '--verify-release-public-downloads' : published ? '--verify-release-published' : '--verify-release-draft'
    const requestEnv = { ...process.env, NODE_OPTIONS: '--import=' + pathToFileURL(githubFetchFixture).href,
      JINDI_CORRUPT_PUBLIC: corruptPublic ? '1' : '' }
    return run(checksums, [option, version, output, draftFile, jobsFile, archiveFile, artifactFile],
      { cwd, env: requestEnv })
  }
  const checkDraft = checkRelease
  const checkPublished = (data) => checkRelease(data, { published: true })
  const validDraft = await checkDraft(draft)
  assert.equal(validDraft.status, 0, validDraft.stderr)
  assert.match(validDraft.stdout, /matches all 11 handoff files/)
  assert.match(validDraft.stdout, /build 12345 attempt 2 matches all 5 successful release jobs and 21 required steps/)
  assert.match(validDraft.stdout, /artifact 321 matches the original ZIP and all 11 handoff files/)
  const missingOrigin = await checkDraft(draft, { tagResponses: { [refURL]: { status: 404 } } })
  assert.equal(missingOrigin.status, 1, 'The local mirror must not stand in for GitHub')
  assert.match(missingOrigin.stderr, /Cannot resolve GitHub release tag/)
  const withChangedJob = (name, update) => jobs.map((job) => job.name === name ? update(job) : job)
  const macSigning = 'package-release (macos-latest)'
  const notarization = 'notarize application (macos release)'
  const changeNotarization = (update) => withChangedJob(macSigning, (job) => ({
    ...job, steps: job.steps.map((step) => step.name === notarization ? { ...step, ...update } : step),
  }))
  for (const [releaseJobs, message] of [
    [changeNotarization({ conclusion: 'skipped' }), /must have succeeded in order: notarize application/],
    [withChangedJob(macSigning, (job) => ({
      ...job, steps: job.steps.filter(({ name }) => name !== 'build and verify deployment target (macos)'),
    })), /Expected one release step named build and verify deployment target/],
    [withChangedJob(macSigning, (job) => ({
      ...job, steps: job.steps.map((step) => step.name === 'zip (macos)' ? { ...step, conclusion: 'skipped' } : step),
    })), /must have succeeded in order: zip/],

    [changeNotarization({ number: 2 }), /must have succeeded in order: notarize application/],
    [changeNotarization({ number: null }), /must have succeeded in order: notarize application/],
    [withChangedJob(macSigning, (job) => ({
      ...job, steps: job.steps.filter(({ name }) => name !== notarization),
    })), /Expected one release step named notarize application/],
    [withChangedJob(macSigning, (job) => ({
      ...job, steps: [...job.steps, job.steps.find(({ name }) => name === notarization)],
    })), /Expected one release step named notarize application/],
    [withChangedJob('verify-macos-intel', (job) => ({
      ...job, steps: null,
    })), /Missing steps for release job verify-macos-intel/],
    [withChangedJob('package-release (windows-latest)', (job) => ({
      ...job, steps: job.steps.map((step) => step.name === 'sign application and installer (windows release)' ?
        { ...step, conclusion: 'skipped' } : step),
    })), /must have succeeded in order: sign application and installer/],
  ]) {
    const result = await checkDraft(draft, { releaseJobs })
    assert.equal(result.status, 1, result.stderr)
    assert.match(result.stderr, message)
  }

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
  for (const jobIndex of [0, 1]) {
    const original = jobs[jobIndex]
    for (const gate of ['verify frontend release regressions', 'verify native Go release regressions']) {
      const omitted = original.steps.filter((step) => step.name !== gate)
      const failed = original.steps.map((step) => step.name === gate ? { ...step, conclusion: 'failure' } : step)
      const skipped = original.steps.map((step) => step.name === gate ? { ...step, conclusion: 'skipped' } : step)
      for (const steps of [omitted, failed, skipped]) {
        const releaseJobs = jobs.map((job, index) => index === jobIndex ? { ...job, steps } : job)
        const rejected = await checkDraft(draft, { releaseJobs })
        assert.equal(rejected.status, 1, rejected.stdout)
        assert.match(rejected.stderr, new RegExp(gate))
      }
    }
    for (const gate of ['verify frontend release regressions', 'verify native Go release regressions']) {
      const nativeBuild = original.steps.find((step) => step.name.startsWith('build '))
      const late = original.steps.map((step) => step.name === gate ? { ...step, number: nativeBuild.number + 1 } : step)
      const releaseJobs = jobs.map((job, index) => index === jobIndex ? { ...job, steps: late } : job)
      const rejected = await checkDraft(draft, { releaseJobs })
      assert.equal(rejected.status, 1, rejected.stdout)
      assert.match(rejected.stderr, /steps must have succeeded in order/)
    }
  }
  const normalizedDraft = await checkDraft({ ...draft, body: body.trimEnd().replace(/\n/g, '\r\n') })
  assert.equal(normalizedDraft.status, 0, normalizedDraft.stderr)
  for (const invalidBody of [null, body + '\nUnverified change']) {
    const rejected = await checkDraft({ ...draft, body: invalidBody })
    assert.equal(rejected.status, 1)
    assert.match(rejected.stderr, /draft body does not match RELEASE-NOTES.md/)
  }

  const moved = await checkDraft(draft, { tagResponses: { [refURL]: { body: { ref, object: { ...commitObject, sha: wrongSHA } } } } })
  assert.equal(moved.status, 1)
  assert.match(moved.stderr, /GitHub release tag does not match the handoff build commit/)
  git(remote, 'update-ref', ref, wrongSHA)
  const remoteLightweight = await checkDraft(draft, { tagResponses: { [refURL]: { body: lightweightReference } } })
  assert.equal(remoteLightweight.status, 0, remoteLightweight.stderr)
  const nestedSHA = 'b'.repeat(40)
  const nestedURL = api + '/git/tags/' + nestedSHA
  const nestedResponses = {
    ...annotatedResponses,
    [annotationURL]: { body: { sha: annotationSHA, object: { type: 'tag', sha: nestedSHA } } },
    [nestedURL]: { body: { sha: nestedSHA, object: commitObject } },
  }
  const remoteNested = await checkDraft(draft, { tagResponses: nestedResponses })
  assert.equal(remoteNested.status, 0, remoteNested.stderr)
  for (const [tagResponses, message] of [
    [{ [refURL]: { body: { ...lightweightReference, ref: 'refs/heads/' + version } } }, /Unexpected GitHub release tag reference/],
    [{ ...annotatedResponses, [annotationURL]: { status: 404 } }, /Cannot resolve GitHub release tag/],
    [{ ...annotatedResponses, [annotationURL]: { body: { sha: nestedSHA, object: commitObject } } }, /Unexpected GitHub release tag object/],
    [{ [refURL]: { body: { ref, object: { ...commitObject, type: 'tree' } } } }, /Unexpected GitHub release tag object/],
    [{ ...nestedResponses, [nestedURL]: { body: { sha: nestedSHA, object: { type: 'tag', sha: annotationSHA } } } }, /Unexpected GitHub release tag object/],
  ]) {
    const result = await checkDraft(draft, { tagResponses })
    assert.equal(result.status, 1)
    assert.match(result.stderr, message)
  }

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
  const publicPage = `https://github.com/${repository}/releases`
  const published = {
    ...draft, draft: false, published_at: '2026-11-27T08:00:00Z',
    html_url: `${publicPage}/tag/${version}`,
    assets: assets.map((asset) => ({
      ...asset,
      url: `https://api.github.com/repos/${repository}/releases/assets/${asset.id}`,
      browser_download_url: `${publicPage}/download/${version}/${asset.name}`,
    })),
  }
  const validPublished = await checkPublished(published)
  assert.equal(validPublished.status, 0, validPublished.stderr)
  assert.match(validPublished.stdout, /GitHub published release 123 matches all 11 handoff files/)
  const publicDownloads = await checkRelease(published, { downloadPublic: true })
  assert.equal(publicDownloads.status, 0, publicDownloads.stderr)
  assert.match(publicDownloads.stdout, /All 11 public downloads match the handoff/)
  assert.match(publicDownloads.stdout, /Public download SHA256SUMS.txt: SHA-256 OK/)
  for (const mode of [{ published: true }, { downloadPublic: true }]) {
    const missingTag = await checkRelease(published, { ...mode, tagResponses: { [refURL]: { status: 404 } } })
    assert.equal(missingTag.status, 1)
    assert.match(missingTag.stderr, /Cannot resolve GitHub release tag/)
    assert.doesNotMatch(missingTag.stdout, /Public download |matches all 11 handoff files/)
  }
  const wrongPublicBytes = await checkRelease(published, { downloadPublic: true, corruptPublic: true })
  assert.equal(wrongPublicBytes.status, 1)
  assert.match(wrongPublicBytes.stderr, /Public asset SHA-256 mismatch: SHA256SUMS.txt/)
  assert.doesNotMatch(wrongPublicBytes.stdout, /All 11 public downloads/)

  assert.match((await checkDraft(published)).stderr, /repository, tag, and release channel/)
  assert.match((await checkPublished(draft)).stderr, /repository, tag, and release channel/)
  assert.match((await checkPublished({ ...published, prerelease: false })).stderr, /repository, tag, and release channel/)
  assert.match((await checkPublished({ ...published, tag_name: 'v0.1.0-rc.2' })).stderr, /repository, tag, and release channel/)
  const encodedPage = await checkPublished({ ...published, html_url: `${publicPage}/tag/%76${version.slice(1)}` })
  assert.equal(encodedPage.status, 0, encodedPage.stderr)
  for (const wrong of [
    { published_at: null }, { published_at: '2026' }, { published_at: '2026-02-30T08:00:00Z' },
    { html_url: `${publicPage}/tag/v0.1.0` },
    { html_url: `${publicPage}/tag/${version}?view=1` },
    { html_url: `${publicPage}/tag/%ZZ` },
  ]) {
    assert.match((await checkPublished({ ...published, ...wrong })).stderr, /publication time or has an unexpected public release page/)
  }
  const changeInstaller = (wrong) => ({
    ...published,
    assets: published.assets.map((asset) => asset.name === 'JindiDrama-amd64-installer.exe' ? { ...asset, ...wrong } : asset),
  })
  for (const wrong of [
    { url: 'https://api.github.com/repos/example/Other/releases/assets/101' },
    { browser_download_url: `${publicPage}/download/${version}/JindiDrama.exe` },
    { browser_download_url: `https://example.com/download/${version}/JindiDrama-amd64-installer.exe` },
  ]) {
    assert.match((await checkPublished(changeInstaller(wrong))).stderr, /download URL mismatch: JindiDrama-amd64-installer.exe/)
  }
  const otherAsset = published.assets.find((asset) => asset.name !== 'JindiDrama-amd64-installer.exe')
  assert.match((await checkPublished(changeInstaller({ id: otherAsset.id, url: otherAsset.url }))).stderr,
    /asset ID, upload state, size or SHA-256 mismatch/)
  assert.match((await checkPublished(changeInstaller({ digest: 'sha256:' + '0'.repeat(64) }))).stderr,
    /SHA-256 mismatch: JindiDrama-amd64-installer.exe/)
  assert.match((await checkPublished({ ...published, assets: published.assets.filter(({ name }) => name !== 'SHA256SUMS.txt') })).stderr,
    /Missing published release asset: SHA256SUMS.txt/)
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
  assert.match(notes, /设置 → 播放 → 苹果 CMS 片源/)
  assert.match(notes, /测试连接.*保存片源.*重新打开/)
  assert.match(notes, /清除片源.*完全退出并重新打开应用.*演示目录/)
  assert.match(notes, /公网 HTTPS.*MP4.*WebM.*未加密的 HLS 点播/s)
  assert.match(notes, /红果官方服务仍待接入授权/)
  assert.match(notes, /发布下载区的隐私说明 `PRIVACY.md`/)
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
