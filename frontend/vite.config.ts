/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'

// The API paths are proxied as they are, without an /api prefix: the
// registration, password reset and Google cookies are scoped to these exact
// paths, so a prefix would stop the browser from sending them.
const apiPaths = '^/(registrations|sessions|me|auth|password-resets|urls|health|docs)(/|$)'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        [apiPaths]: {
          target: env.VITE_API_PROXY_TARGET || 'http://localhost:8080',
        },
      },
    },
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      restoreMocks: true,
    },
  }
})
