<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ChevronLeft, ChevronRight, Play, Plus, Check } from 'lucide-vue-next'
import type { Banner } from '@/types'
import { posterSvg } from '@/mocks/posters'
import BaseButton from './ui/BaseButton.vue'

const props = defineProps<{ banners: Banner[]; autoplay?: boolean; isFav: (dramaId: string) => boolean }>()
const emit = defineEmits<{ watch: [dramaId: string]; fav: [dramaId: string] }>()
const idx = ref(0)
const cur = computed(() => props.banners[idx.value])
const art = computed(() => (cur.value ? posterSvg({ hue: cur.value.hue, hue2: cur.value.hue2, seed: cur.value.seed, wide: true }) : ''))
const go = (d: number) => { idx.value = (idx.value + d + props.banners.length) % props.banners.length }
let timer: number | undefined
const paused = ref(false)
onMounted(() => { if (props.autoplay !== false) timer = window.setInterval(() => { if (!paused.value) go(1) }, 6000) })
onBeforeUnmount(() => clearInterval(timer))
</script>
<template>
  <section v-if="cur" class="hero" aria-roledescription="carousel" aria-label="今日推荐" data-testid="hero" @mouseenter="paused = true" @mouseleave="paused = false" @keydown.left="go(-1)" @keydown.right="go(1)">
    <div class="art" :style="{ backgroundImage: `url('${art}')` }" />
    <div class="veil" />
    <div class="copy">
      <div class="tags"><span v-for="(t, i) in cur.tags" :key="t">{{ t }}<i v-if="i < cur.tags.length - 1"> · </i></span></div>
      <h2 class="title">{{ cur.title }}</h2>
      <p class="sub">{{ cur.subtitle }}</p>
      <div class="cta">
        <BaseButton size="lg" data-testid="hero-watch" @click="emit('watch', cur.dramaId)"><Play :size="17" fill="currentColor" />立即观看</BaseButton>
        <BaseButton size="lg" variant="ghost" data-testid="hero-fav" @click="emit('fav', cur.dramaId)"><component :is="isFav(cur.dramaId) ? Check : Plus" :size="18" />{{ isFav(cur.dramaId) ? '已收藏' : '收藏' }}</BaseButton>
      </div>
    </div>
    <button class="arrow l" aria-label="上一张" data-testid="hero-prev" @click="go(-1)"><ChevronLeft :size="20" /></button>
    <button class="arrow r" aria-label="下一张" data-testid="hero-next" @click="go(1)"><ChevronRight :size="20" /></button>
    <div class="dots" role="tablist"><button v-for="(b, i) in banners" :key="b.id" role="tab" :aria-selected="i === idx" :aria-label="`第${i + 1}张`" :class="{ on: i === idx }" @click="idx = i" /></div>
  </section>
</template>
<style scoped lang="scss">
.hero { position: relative; height: 294px; border-radius: 24px; overflow: hidden; background: #2a1517; border: 1px solid var(--c-border); }
.art { position: absolute; inset: 0; background-size: cover; background-position: right center; filter: saturate(1.1); }
.veil { position: absolute; inset: 0; background: linear-gradient(90deg, rgba(25,10,12,.92) 0%, rgba(25,10,12,.7) 32%, rgba(25,10,12,0) 62%), linear-gradient(0deg, rgba(25,10,12,.5), transparent 40%); }
.copy { position: absolute; left: 68px; top: 24px; width: 520px; }
.tags { display: inline-block; padding: 4px 14px; border-radius: 14px; background: rgba(255,255,255,.1); border: 1px solid rgba(255,255,255,.12); font-size: 12px; color: var(--c-text-2); i { font-style: normal; } }
.title { margin-top: 8px; font-family: var(--font-display); font-weight: 400; font-size: 48px; line-height: 1.08; white-space: pre-line; letter-spacing: 0; text-shadow: 0 2px 12px rgba(0,0,0,.5); }
.sub { margin-top: 12px; font-size: 14.5px; line-height: 1.55; color: var(--c-text-2); white-space: pre-line; }
.cta { display: flex; gap: 16px; margin-top: 14px; }
.arrow { position: absolute; top: 50%; transform: translateY(-50%); width: 36px; height: 36px; border-radius: 50%; background: rgba(40,20,22,.7); border: 1px solid var(--c-border-strong); display: grid; place-items: center; &:hover { background: rgba(80,30,30,.85); } &.l { left: 14px; } &.r { right: 14px; } }
.dots { position: absolute; right: 36px; bottom: 16px; display: flex; gap: 8px; button { width: 10px; height: 8px; border-radius: 6px; background: rgba(255,255,255,.4); &.on { width: 14px; background: #fff; } } }
</style>
