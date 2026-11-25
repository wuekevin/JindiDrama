import { describe, expect, it } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import HomeView from './HomeView.vue'

async function setup(path = '/home?static') {
  localStorage.clear()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/home', component: HomeView }, { path: '/:p(.*)*', component: { template: '<i/>' } }] })
  router.push(path); await router.isReady()
  const w = mount(HomeView, { global: { plugins: [createPinia(), router] } })
  await new Promise((r) => setTimeout(r, 200)); await flushPromises()
  return { w, router }
}
describe('HomeView', () => {
  it('renders sections', async () => {
    const { w } = await setup()
    expect(w.findAll('[data-testid=drama-card]')).toHaveLength(6)
    expect(w.findAll('[data-testid=rank-item]')).toHaveLength(5)
    expect(w.findAll('[data-testid=resume-card]')).toHaveLength(3)
  })
  it('switches ranking tab', async () => {
    const { w } = await setup()
    const first = w.find('[data-testid=rank-item]').text()
    await w.find('[data-testid=rank-rise]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=rank-item]').text()).not.toBe(first)
  })
  it('shows error state with retry', async () => { const { w } = await setup('/home?state=error'); expect(w.find('[data-testid=error]').exists()).toBe(true) })
  it('shows empty state', async () => { const { w } = await setup('/home?state=empty'); expect(w.find('[data-testid=empty]').exists()).toBe(true) })
  it('navigates to drama on card click', async () => { const { w, router } = await setup(); await w.find('[data-testid=drama-card]').trigger('click'); await flushPromises(); expect(router.currentRoute.value.path).toMatch(/^\/drama\//) })
})
