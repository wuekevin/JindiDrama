import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

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

async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'jindi-release-handoff-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  const windows = join(root, 'windows')
  const mac = join(root, 'macos')
  const output = join(root, 'release')
  for (const [platform, dir] of [['windows', windows], ['macos', mac]]) {
    await mkdir(dir)
    await writeFile(join(dir, 'RELEASE-NOTES.md'), releaseNotes)
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

test('assembles both verified platforms and emits one complete release manifest', async (t) => {
  const { output, args } = await fixture(t)
  const result = run(assemble, args, { env })
  assert.equal(result.status, 0, result.stderr)
  const check = run(checksums, ['--verify', 'release', output])
  assert.equal(check.status, 0, check.stderr)
  assert.equal(check.stdout.trim().split('\n').length, 10)
  assert.equal(await readFile(join(output, 'GO-LICENSES-windows.md'), 'utf8'), 'go licenses windows')
  assert.equal(await readFile(join(output, 'GO-LICENSES-macos.md'), 'utf8'), 'go licenses macos')
  assert.match(await readFile(join(output, 'RELEASE-NOTES.md'), 'utf8'), /打包运行 12345/)
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
