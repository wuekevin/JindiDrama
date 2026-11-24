import { createHash } from 'node:crypto'
import { createReadStream } from 'node:fs'
import { readdir, writeFile } from 'node:fs/promises'
import { join } from 'node:path'

const directory = process.argv[2]
if (!directory) {
  throw new Error('Usage: node scripts/package-checksums.mjs <package-directory>')
}

const entries = await readdir(directory, { withFileTypes: true })
// Match the upload step's visible files and Windows glob casing.
const packageExtension = process.platform === 'win32' ? /\.(exe|zip)$/i : /\.(exe|zip)$/
const packages = entries
  .filter(entry => entry.isFile() && !entry.name.startsWith('.') && packageExtension.test(entry.name))
  .map(entry => entry.name)
  .sort()

if (packages.length === 0) {
  throw new Error(`No .exe or .zip packages found in ${directory}`)
}

const lines = []
for (const name of packages) {
  const hash = createHash('sha256')
  for await (const chunk of createReadStream(join(directory, name))) {
    hash.update(chunk)
  }
  lines.push(`${hash.digest('hex')}  ${name}`)
}

await writeFile(join(directory, 'SHA256SUMS.txt'), lines.join('\n') + '\n')
