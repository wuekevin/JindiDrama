import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath, pathToFileURL } from 'node:url'

import { verifyPublicReleaseAsset } from './package-checksums.mjs'

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


const publicAPI = 'https://api.github.com/repos/wuekevin/JindiDrama/releases/assets/1'
const publicCDN = 'https://release-assets.githubusercontent.com/asset?opaque=1'
const payload = Buffer.from('verified package')
const publicAsset = {
  name: 'SHA256SUMS.txt',
  size: payload.length,
  digest: createHash('sha256').update(payload).digest('hex'),
}

test('verifies streamed public bytes over GitHub redirects without authentication', async () => {
  const visited = []
  await verifyPublicReleaseAsset(publicAPI, publicAsset, async (url, options) => {
    assert.equal(options.credentials, 'omit')
    assert.equal(options.redirect, 'manual')
    assert.equal(options.headers.Accept, 'application/octet-stream')
    assert.equal(options.headers.Authorization, undefined)
    visited.push(url.href)
    if (url.href === publicAPI) return Response.redirect(publicCDN, 302)
    return new Response(new ReadableStream({
      start(controller) {
        controller.enqueue(payload.subarray(0, 4))
        controller.enqueue(payload.subarray(4))
        controller.close()
      },
    }))
  })
  assert.deepEqual(visited, [publicAPI, publicCDN])
})

test('rejects public assets with an incorrect response, reduced or extended size, or different bytes', async () => {
  for (const [response, message] of [
    [() => new Response(null, { status: 404 }), /response \(404\)/],
    [() => new Response(payload, { headers: { 'content-encoding': 'gzip' } }), /response \(200\)/],
    [() => new Response(payload, { headers: { 'content-length': '9000' } }), /size mismatch/],
    [() => new Response(payload.subarray(1)), /size mismatch/],
    [() => new Response(Buffer.concat([payload, payload])), /size mismatch/],
    [() => new Response(Buffer.alloc(payload.length)), /SHA-256 mismatch/],
    [() => new Response(new ReadableStream({ start(controller) { controller.error(new Error('signed-url-detail')) } })),
      /Incomplete public asset transfer: SHA256SUMS.txt/],
  ]) {
    await assert.rejects(verifyPublicReleaseAsset(publicAPI, publicAsset, async () => response()), message)
  }
})

test('rejects public asset redirects outside the GitHub CDN or above the redirect limit', async () => {
  for (const url of ['https://example.test/assets/1', publicAPI + '?token=secret']) {
    await assert.rejects(verifyPublicReleaseAsset(url, publicAsset, async () => {
      assert.fail('invalid initial URL reached the transport')
    }), /Invalid public asset API URL/)
  }
  for (const location of [
    'http://release-assets.githubusercontent.com/asset',
    'https://release-assets.githubusercontent.com:443/asset',
    'https://release-assets.githubusercontent.com:444/asset',
    'https://release-assets.githubusercontent.com.evil.test/asset',
    'https://account:password@release-assets.githubusercontent.com/asset',
    '/relative',
  ]) {
    let calls = 0
    await assert.rejects(verifyPublicReleaseAsset(publicAPI, publicAsset, async () => {
      calls++
      return new Response(null, { status: 302, headers: { location } })
    }), /Unexpected public asset redirect/)
    assert.equal(calls, 1)
  }
  let calls = 0
  await assert.rejects(verifyPublicReleaseAsset(publicAPI, publicAsset, async () => {
    calls++
    return Response.redirect(publicCDN)
  }), /Unexpected public asset redirect/)
  assert.equal(calls, 4)
})
