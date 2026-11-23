// 用法: node tools/diff.mjs 01-首页推荐   → 对比 design/reference 与 design/rendered，输出 design/diff/<id>-diff.png 与指标
import fs from 'node:fs'; import { PNG } from 'pngjs'; import pixelmatch from 'pixelmatch'
const id = process.argv[2]; const root = new URL('../../design/', import.meta.url).pathname
const a = PNG.sync.read(fs.readFileSync(`${root}reference/${id}.png`)); const b = PNG.sync.read(fs.readFileSync(`${root}rendered/${id}.png`))
if (a.width !== b.width || a.height !== b.height) { console.log(`SIZE MISMATCH ${a.width}x${a.height} vs ${b.width}x${b.height}`); process.exit(1) }
const d = new PNG({ width: a.width, height: a.height })
const n = pixelmatch(a.data, b.data, d.data, a.width, a.height, { threshold: 0.2 })
fs.writeFileSync(`${root}diff/${id.slice(0, 2)}-diff.png`, PNG.sync.write(d))
// 结构分：32px 块平均亮度差（对海报内容不敏感，反映布局/明暗层次）
const lum = (img, x, y) => { const i = (y * img.width + x) * 4; return 0.299 * img.data[i] + 0.587 * img.data[i + 1] + 0.114 * img.data[i + 2] }
let err = 0, cnt = 0; const B = 32
for (let by = 0; by < a.height - B; by += B) for (let bx = 0; bx < a.width - B; bx += B) { let sa = 0, sb = 0; for (let y = 0; y < B; y += 4) for (let x = 0; x < B; x += 4) { sa += lum(a, bx + x, by + y); sb += lum(b, bx + x, by + y) } err += Math.abs(sa - sb) / 64; cnt++ }
console.log(JSON.stringify({ id, size: `${a.width}x${a.height}`, pixelMismatchPct: +(100 * n / (a.width * a.height)).toFixed(2), blockLumaMAE: +(err / cnt).toFixed(2) }))
