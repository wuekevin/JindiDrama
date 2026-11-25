import { defineConfig } from '@playwright/test'

const requestedPort = process.env.JINDI_E2E_PORT
const port = Number(requestedPort ?? '4173')
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error('JINDI_E2E_PORT must be a valid TCP port')
}
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30000,
  workers: 1,
  reporter: [['list']],
  use: { baseURL, viewport: { width: 1448, height: 1086 }, deviceScaleFactor: 1, trace: 'off' },
  webServer: {
    command: `npm run build && node ./node_modules/vite/bin/vite.js preview --host 127.0.0.1 --port ${port} --strictPort`,
    url: baseURL,
    reuseExistingServer: !process.env.CI && requestedPort === undefined,
    timeout: 120000,
  },
})
