import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/docs': 'http://localhost:8080',
      '/openapi.yaml': 'http://localhost:8080',
      '/openapi.json': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../backend/web/dist',
    emptyOutDir: true,
  },
})

