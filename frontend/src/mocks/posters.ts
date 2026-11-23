// 原创程序化海报：渐变 + 散景 + 人物剪影。非任何影视作品素材，替代设计稿中的示例海报（偏差已记录于 design/diff）。
export interface PosterSpec { hue: number; hue2: number; seed: number; style?: 'warm' | 'cool' | 'night' | 'bright'; wide?: boolean }

function rnd(seed: number) {
  let s = seed * 9301 + 49297
  return () => { s = (s * 9301 + 49297) % 233280; return s / 233280 }
}

export function posterSvg(spec: PosterSpec): string {
  const r = rnd(spec.seed)
  const { hue, hue2 } = spec
  const dark = spec.style === 'night' ? 18 : spec.style === 'bright' ? 44 : 28
  let bokeh = ''
  for (let i = 0; i < 9; i++) {
    bokeh += `<circle cx="${r() * (spec.wide ? 760 : 300)}" cy="${r() * 260}" r="${8 + r() * 26}" fill="hsl(${hue2} 80% 70%)" opacity="${0.1 + r() * 0.22}"/>`
  }
  const skin = 'hsl(24 55% 78%)'
  const hair = `hsl(${(hue + 200) % 360} 25% 12%)`
  const suit = `hsl(${hue} 25% ${dark - 12}%)`
  const dress = `hsl(${hue2} 45% ${spec.style === 'bright' ? 82 : 55}%)`
  const flip = spec.seed % 2 === 0
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${spec.wide ? '0 0 760 294' : '0 0 300 420'}" preserveAspectRatio="xMidYMid slice">
<defs>
<linearGradient id="b" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue} 55% ${dark + 8}%)"/><stop offset="1" stop-color="hsl(${hue2} 60% ${dark - 6}%)"/></linearGradient>
<radialGradient id="g" cx="0.5" cy="0.35" r="0.7"><stop offset="0" stop-color="hsl(${hue2} 90% 70%)" stop-opacity="0.45"/><stop offset="1" stop-color="#000" stop-opacity="0"/></radialGradient>
</defs>
<rect width="760" height="420" fill="url(#b)"/><rect width="760" height="420" fill="url(#g)"/>${bokeh}
<g transform="${spec.wide ? 'translate(400 -62) scale(0.85)' : flip ? 'translate(300 0) scale(-1 1)' : ''}">
<path d="M-10 420 C 20 300 80 270 130 270 C 180 270 215 300 230 420 Z" fill="${dress}"/>
<ellipse cx="128" cy="198" rx="37" ry="45" fill="${skin}"/>
<path d="M88 190 C 85 140 130 128 160 148 C 175 160 172 190 168 200 C 160 170 120 160 100 190 C 102 230 90 250 80 260 C 70 230 80 210 88 190Z" fill="${hair}"/>
<path d="M150 420 C 160 330 190 300 225 300 C 265 300 285 340 300 420 Z" fill="${suit}"/>
<ellipse cx="222" cy="232" rx="33" ry="40" fill="${skin}"/>
<path d="M190 226 C 186 190 222 180 248 198 C 258 208 256 228 254 238 C 246 214 214 208 198 232Z" fill="${hair}"/>
</g>
<rect y="210" width="300" height="210" fill="#000" opacity="0.0"/>
</svg>`
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
}
