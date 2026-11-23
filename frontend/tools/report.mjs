// 用法: node tools/report.mjs <id> "<标题>" "<已对齐>" "<偏差1;偏差2>" "<验证记录>"   生成 design/diff/XX-report.md
import fs from 'node:fs'; import { execFileSync } from 'node:child_process'
const [, , id, title, aligned, devs, verified] = process.argv; const nn = id.slice(0, 2)
const m = JSON.parse(execFileSync('node', ['tools/diff.mjs', id]).toString())
const md = `# ${nn} ${title} — 差异报告
- 参考：\`design/reference/${id}.png\`（1448×1086） 实际：\`design/rendered/${id}.png\`（Playwright Chromium，viewport 1448×1086，DPR 1）
- 差异图：\`design/diff/${nn}-diff.png\`；响应式截图：\`design/rendered/responsive/${nn}-{1366x768,1920x1080,2560x1440}.png\`
- 指标（\`node tools/diff.mjs ${id}\`，生成于 ${new Date().toISOString().slice(0, 10)}）：像素不一致 ${m.pixelMismatchPct}%（主要来自海报/人物图像与字体渲染差异）；32px 块亮度 MAE ${m.blockLumaMAE}。仅作辅助，结论以人工几何核对为准。

## 已对齐
${aligned}

## 已记录偏差
${devs.split(';').map((x, i) => `${i + 1}. ${x.trim()}`).join('\n')}
通用：海报为原创程序化 SVG；品牌名为“金帝短剧”；图标为 Lucide；字体使用系统中文字体。

## 验证记录（macOS arm64，Chromium，Mock 数据）
${verified}
`
fs.writeFileSync(`../design/diff/${nn}-report.md`, md); console.log('written', nn)
