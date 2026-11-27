import { createHash } from 'node:crypto'
import { readdir } from 'node:fs/promises'
import { join, relative, sep } from 'node:path'
import { hashFile } from './package-checksums.mjs'

const [directory, ...extra] = process.argv.slice(2)
if (!directory || extra.length) {
  throw new Error('Usage: node scripts/package-frontend-digest.mjs <frontend-dist>')
}

const entries = await readdir(directory, { recursive: true, withFileTypes: true })
const files = new Map()
for (const entry of entries) {
  if (entry.isDirectory()) continue
  const path = join(entry.parentPath, entry.name)
  const name = relative(directory, path).split(sep).join('/')
  if (!entry.isFile()) throw new Error('Expected a regular frontend file: ' + name)
  files.set(name, path)
}
if (!files.has('index.html')) throw new Error('Frontend build is missing index.html')

const digest = createHash('sha256')
for (const name of [...files.keys()].sort()) {
  digest.update(JSON.stringify([name, await hashFile(files.get(name))]))
}
console.log(digest.digest('hex'))
