import { statSync } from 'node:fs'
import { defineConfig } from '@playwright/test'

const requestedPort = process.env.JINDI_E2E_PORT
const port = Number(requestedPort ?? '4173')
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error('JINDI_E2E_PORT must be a valid TCP port')
}
const baseURL = `http://127.0.0.1:${port}`
const usePrebuilt = process.env.JINDI_E2E_PREBUILT === '1'
if (usePrebuilt && !statSync(new URL('./dist/index.html', import.meta.url), { throwIfNoEntry: false })?.isFile()) {
  throw new Error('JINDI_E2E_PREBUILT=1 requires frontend/dist/index.html. Run npm run build first.')
}
const preview = `node ./node_modules/vite/bin/vite.js preview --host 127.0.0.1 --port ${port} --strictPort`

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30000,
  workers: 1,
  reporter: [['list']],
  use: { baseURL, viewport: { width: 1448, height: 1086 }, deviceScaleFactor: 1, trace: 'off' },
  webServer: {
    command: usePrebuilt ? preview : `npm run build && ${preview}`,
    // With a prebuilt bundle, wait for this process to bind the port.
    ...(usePrebuilt ? { wait: { stdout: /\bLocal:/ } } : { url: baseURL }),
    reuseExistingServer: !usePrebuilt && !process.env.CI && requestedPort === undefined,
    timeout: 120000,
  },
})
