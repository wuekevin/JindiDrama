import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30000,
  workers: 1,
  reporter: [['list']],
  use: { baseURL: 'http://127.0.0.1:4173', viewport: { width: 1448, height: 1086 }, deviceScaleFactor: 1, trace: 'off' },
  webServer: { command: 'npm run build && npm run preview', url: 'http://127.0.0.1:4173', reuseExistingServer: true, timeout: 120000 },
})
