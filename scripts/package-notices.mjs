import { execFileSync } from 'node:child_process'
import { copyFile, readFile, readdir, writeFile } from 'node:fs/promises'
import { join } from 'node:path'

const [directory, platform = process.platform] = process.argv.slice(2)
if (!directory || (platform !== 'darwin' && platform !== 'win32')) {
  throw new Error('Usage: node scripts/package-notices.mjs <package-directory> [darwin|win32]')
}

const targets = platform === 'darwin' ? ['amd64', 'arm64'] : ['amd64']
const goOS = platform === 'win32' ? 'windows' : 'darwin'
const format = '{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}'
const modules = new Map()
for (const arch of targets) {
  const env = { ...process.env, GOOS: goOS, GOARCH: arch, CGO_ENABLED: goOS === 'darwin' ? '1' : '0' }
  const list = execFileSync('go', ['list', '-deps', '-tags', 'desktop,production', '-f', format, '.'], { env, encoding: 'utf8' })
  for (const row of list.split(/\r?\n/).filter(Boolean)) {
    const [name, version, root] = row.split('\t')
    if (!version || !root) throw new Error(`Missing Go module metadata: ${name}`)
    modules.set(`${name} ${version}`, root)
  }
}
if (!modules.size) throw new Error('No Go modules found in the desktop build')

const goVersion = execFileSync('go', ['env', 'GOVERSION'], { encoding: 'utf8' }).trim()
const goRoot = execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8' }).trim()
modules.set(`Go standard library ${goVersion}`, goRoot)
const goVendor = join(goRoot, 'src', 'vendor')
const vendorList = await readFile(join(goVendor, 'modules.txt'), 'utf8')
for (const [, name, version] of vendorList.matchAll(/^# (\S+) (v\S+)$/gm)) {
  modules.set(`${name} ${version} (bundled with Go)`, join(goVendor, name))
}

const sections = ['# Go 第三方许可与声明', `目标：${goOS}/${targets.join(', ')}`]
for (const name of [...modules.keys()].sort()) {
  const root = modules.get(name)
  const notices = (await readdir(root, { withFileTypes: true }))
    .filter((entry) => entry.isFile() && /^(licen[cs]e|copying|notice|copyright|patents)([._-].*)?$/i.test(entry.name))
    .map(({ name }) => name)
    .sort()
  if (!notices.some((name) => /^(licen[cs]e|copying)/i.test(name))) {
    throw new Error(`No license text found for ${name}`)
  }
  sections.push(`## ${name}`)
  for (const notice of notices) {
    sections.push(`### ${notice}`, (await readFile(join(root, notice), 'utf8')).trim())
  }
}

const frontend = JSON.parse(await readFile('frontend/dist/.vite/frontend-licenses.json', 'utf8'))
if (!Array.isArray(frontend) || !frontend.length) throw new Error('Missing bundled frontend license inventory')
const frontendNotices = ['# 前端第三方许可']
for (const dependency of frontend) {
  if (!dependency.name || !dependency.version || !dependency.identifier || !dependency.text?.trim()) {
    throw new Error(`Missing frontend license metadata or text: ${dependency.name ?? '(unknown)'}`)
  }
  frontendNotices.push(`## ${dependency.name} ${dependency.version} (${dependency.identifier})`, dependency.text.trim())
}
await copyFile('docs/PRIVACY.md', join(directory, 'PRIVACY.md'))
await writeFile(join(directory, 'FRONTEND-LICENSES.md'), frontendNotices.join('\n\n') + '\n')
await writeFile(join(directory, 'GO-LICENSES.md'), sections.join('\n\n') + '\n')
