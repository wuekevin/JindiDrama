import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath, pathToFileURL } from 'node:url'

const script = fileURLToPath(new URL('./package-checksums.mjs', import.meta.url))
const installer = 'JindiDrama-amd64-installer.exe'
const manifest = 'SHA256SUMS.txt'
const documents = ['FRONTEND-LICENSES.md', 'GO-LICENSES.md', 'PRIVACY.md', 'RELEASE-NOTES.md', 'RELEASE-ROLLBACK.md']

function runNode(...args) {
  const result = spawnSync(process.execPath, args, { encoding: 'utf8' })
  if (result.error) throw result.error
  return result
}

function run(...args) {
  return runNode(script, ...args)
}

async function fixture(t, platform) {
  const directory = await mkdtemp(join(tmpdir(), 'jindi-checksums-'))
  t.after(() => rm(directory, { recursive: true, force: true }))
  const packages = platform === 'windows' ? [installer, 'JindiDrama.exe'] : ['JindiDrama-macos-universal.zip']
  for (const name of [...documents, ...packages]) await writeFile(join(directory, name), name)
  const generation = run(directory)
  assert.equal(generation.status, 0, generation.stderr)
  return directory
}

for (const platform of ['macos', 'windows']) {
  test(`verifies downloaded ${platform} output from the existing generator`, async (t) => {
    const directory = await fixture(t, platform)
    const verification = run('--verify', platform, directory)
    assert.equal(verification.status, 0, verification.stderr)
    assert.equal(verification.stdout.trim().split('\n').length, platform === 'windows' ? 7 : 6)
  })
}

test('rejects a missing installer even when the manifest is regenerated', async (t) => {
  const directory = await fixture(t, 'windows')
  await rm(join(directory, installer))
  assert.equal(run(directory).status, 0)
  const result = run('--verify', 'windows', directory)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /Missing artifact file: JindiDrama-amd64-installer.exe/)
})

test('rejects a changed file and an unexpected uploaded file', async (t) => {
  const directory = await fixture(t, 'macos')
  await writeFile(join(directory, 'PRIVACY.md'), 'changed')
  let result = run('--verify', 'macos', directory)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /SHA-256 mismatch: PRIVACY.md/)
  await writeFile(join(directory, 'elsewhere.exe'), '')
  result = run('--verify', 'macos', directory)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /Unexpected artifact entry: "elsewhere.exe"/)
})

test('rejects missing, duplicate, and path-based manifest entries', async (t) => {
  const directory = await fixture(t, 'windows')
  const path = join(directory, manifest)
  const [first, ...rest] = (await readFile(path, 'utf8')).trimEnd().split('\n')
  for (const [lines, pattern] of [
    [rest, /Missing checksum for FRONTEND-LICENSES.md/],
    [[first, first, ...rest], /Unexpected or duplicate checksum entry/],
    [[first.replace('FRONTEND-LICENSES.md', '../FRONTEND-LICENSES.md'), ...rest], /Unexpected or duplicate checksum entry/],
  ]) {
    await writeFile(path, lines.join('\n') + '\n')
    const result = run('--verify', 'windows', directory)
    assert.equal(result.status, 1)
    assert.match(result.stderr, pattern)
  }
})

async function linkedEntry(t) {
  const directory = await mkdtemp(join(tmpdir(), 'jindi-checksum-entry-'))
  t.after(() => rm(directory, { recursive: true, force: true }))
  const alias = join(directory, 'cli')
  await symlink(dirname(script), alias, process.platform === 'win32' ? 'junction' : 'dir')
  return join(alias, 'package-checksums.mjs')
}

test('executes generation and verification through a linked directory', async (t) => {
  const entry = await linkedEntry(t)
  const packageDirectory = await fixture(t, 'windows')
  await rm(join(packageDirectory, manifest))

  const generation = runNode(entry, packageDirectory)
  assert.equal(generation.status, 0, generation.stderr)
  assert.match(await readFile(join(packageDirectory, manifest), 'utf8'), /  JindiDrama\.exe/)
  const verification = runNode(entry, '--verify', 'windows', packageDirectory)
  assert.equal(verification.status, 0, verification.stderr)
  assert.equal(verification.stdout.trim().split('\n').length, 7)

  await writeFile(join(packageDirectory, 'PRIVACY.md'), 'changed')
  const rejection = runNode(entry, '--verify', 'windows', packageDirectory)
  assert.equal(rejection.status, 1)
  assert.match(rejection.stderr, /SHA-256 mismatch: PRIVACY.md/)
})

test('exports verification without running the CLI on import', async (t) => {
  const program = `const { verify } = await import(${JSON.stringify(pathToFileURL(script).href)}); if (typeof verify !== 'function') process.exit(1)`
  for (const argument of ['missing-entry-for-import', await linkedEntry(t), script]) {
    const result = runNode('--input-type=module', '-e', program, '--', argument)
    assert.equal(result.status, 0, result.stderr)
    assert.equal(result.stdout, '')
  }
})
