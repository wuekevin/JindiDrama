// 用法: node tools/shot.mjs <route> <out.png> [w] [h]   (需先 npm run build && npm run preview)
import { chromium } from '@playwright/test'
const [, , route, out, w = '1448', h = '1086'] = process.argv
const b = await chromium.launch(); const p = await b.newPage({ viewport: { width: +w, height: +h }, deviceScaleFactor: 1 })
await p.goto(`http://127.0.0.1:4173/#${route}`); await p.waitForTimeout(700)
await p.screenshot({ path: out }); await b.close()
