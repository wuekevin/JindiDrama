import { createRouter, createWebHashHistory } from 'vue-router'
import { hasBridge, LS } from '@/bindings'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { beginStartupPlayback, unfinishedBookmark } from './libraryRoutes'
import { useAppSettings } from '@/stores/appSettings'
import { useLibraryStore } from '@/stores/library'

function startupRoute() {
  if (!hasBridge()) return '/home'
  const page = useAppSettings().s.startPage
  if (page === '我的收藏') return '/favorites'
  if (page !== '上次观看') return '/home'
  try {
    // A fresh desktop install may show demonstration history. Restore only saved history.
    if (!localStorage.getItem(LS.library)) return '/home'
  } catch { return '/home' }
  const { card } = useLicensedCatalog()
  const recent = useLibraryStore().history.filter((entry) => {
    const drama = card(entry.dramaId)
    return !!drama && unfinishedBookmark(drama, entry)
  })
  return beginStartupPlayback(recent)
}
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: startupRoute },
    { path: '/home', name: 'home', component: () => import('@/views/HomeView.vue'), meta: { nav: 'home' } },
    { path: '/licensed', name: 'licensed', component: () => import('@/views/LicensedLibraryView.vue'), meta: { nav: 'home' } },
    { path: '/search', name: 'search', component: () => import('@/views/SearchView.vue'), meta: { nav: 'home' } },
    { path: '/drama/:id', name: 'drama', component: () => import('@/views/DramaDetailView.vue'), meta: { nav: 'home' } },
    { path: '/watch/:dramaId/:episodeId', name: 'watch', component: () => import('@/views/WatchView.vue'), meta: { nav: 'home' } },
    { path: '/favorites', name: 'favorites', component: () => import('@/views/FavoritesView.vue'), meta: { nav: 'favorites' } },
    { path: '/history', name: 'history', component: () => import('@/views/HistoryView.vue'), meta: { nav: 'history' } },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { nav: 'home' } },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { nav: 'settings' } },
  ],
})
