<template>
  <div class="brand" aria-label="金帝短剧">
    <svg class="mark" viewBox="0 0 40 36" width="40" height="36" aria-hidden="true">
      <defs><linearGradient id="lg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ff6a4d"/><stop offset="1" stop-color="#d8301f"/></linearGradient></defs>
      <path d="M4 4 L34 18 L4 32 Z" fill="none" stroke="url(#lg)" stroke-width="5" stroke-linejoin="round"/>
      <path d="M13 12 L26 18 L13 24 Z" fill="url(#lg)"/>
    </svg>
    <div><div class="name">金帝短剧</div><div class="slogan">高能短剧 · 随时开刷</div></div>
  </div>
</template>
<style scoped lang="scss">
.brand { display: flex; align-items: center; gap: 8px; }
.name { font-size: 25px; font-weight: 800; letter-spacing: 0; line-height: 1.1; }
.slogan { font-size: 12px; color: var(--c-text-2); margin-top: 3px; letter-spacing: 0.5px; }
</style>
