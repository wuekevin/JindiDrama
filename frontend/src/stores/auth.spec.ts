import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore, isValidPhone, isValidCode } from './auth'
describe('auth (demo)', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  it('validates phone/code', () => { expect(isValidPhone('13800138000')).toBe(true); expect(isValidPhone('12345678901')).toBe(false); expect(isValidPhone('1380013800')).toBe(false); expect(isValidCode('123456')).toBe(true); expect(isValidCode('12345')).toBe(false); expect(isValidCode('abcdef')).toBe(false) })
  it('demo login/logout holds no token', () => { const a = useAuthStore(); a.loginDemo('13800138000'); expect(a.demo).toBe(true); expect(a.name).toBe('演示用户8000'); a.logout(); expect(a.demo).toBe(false); expect(Object.keys(a.$state)).not.toContain('token') })
})
