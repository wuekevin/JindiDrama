import { createRouter, createWebHashHistory } from 'vue-router'
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/home' },
    { path: '/home', name: 'home', component: () => import('@/views/HomeView.vue'), meta: { nav: 'home' } },
    { path: '/search', name: 'search', component: () => import('@/views/SearchView.vue'), meta: { nav: 'home' } },
    { path: '/drama/:id', name: 'drama', component: () => import('@/views/DramaDetailView.vue'), meta: { nav: 'home' } },
    { path: '/watch/:dramaId/:episodeId', name: 'watch', component: () => import('@/views/WatchView.vue'), meta: { nav: 'home' } },
    { path: '/favorites', name: 'favorites', component: () => import('@/views/FavoritesView.vue'), meta: { nav: 'favorites' } },
    { path: '/history', name: 'history', component: () => import('@/views/HistoryView.vue'), meta: { nav: 'history' } },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { nav: 'home' } },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { nav: 'settings' } },
  ],
})
