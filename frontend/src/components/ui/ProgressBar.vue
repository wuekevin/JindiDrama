<script setup lang="ts">defineProps<{ value: number }>()</script>
<template><div class="bar" role="progressbar" :aria-valuenow="value" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: Math.max(0, Math.min(100, value)) + '%' }" /></div></template>
<style scoped lang="scss">
.bar { height: 5px; border-radius: 5px; background: rgba(255,255,255,.18); overflow: hidden; i { display: block; height: 100%; border-radius: 5px; background: linear-gradient(90deg, #ff8a66, #ee4b39); } }
</style>
