<script setup lang="ts">
import AppSidebar from '@/components/AppSidebar.vue'
import AppTopbar from '@/components/AppTopbar.vue'
import ToastHost from '@/components/ToastHost.vue'
</script>
<template>
  <div class="shell">
    <AppSidebar />
    <div class="main">
      <AppTopbar />
      <main class="content" id="content"><RouterView /></main>
    </div>
    <ToastHost />
  </div>
</template>
<style scoped lang="scss">
.shell { display: flex; width: 100%; height: 100%; background: var(--c-bg-0); overflow: hidden; }
.main { position: relative; flex: 1; min-width: 0; display: flex; flex-direction: column; border-radius: 26px 0 0 26px; background: radial-gradient(900px 500px at 100% 0%, rgba(120, 28, 24, 0.28), transparent 70%), var(--c-bg-1); box-shadow: -1px 0 0 rgba(255, 255, 255, 0.05); overflow: hidden; }
.content { flex: 1; min-height: 0; overflow-y: auto; overflow-x: hidden; padding: 0 14px 12px; }
</style>
