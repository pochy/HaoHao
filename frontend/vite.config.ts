import vue from '@vitejs/plugin-vue'
import { mkdir, readdir, rm, copyFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'

const repoRoot = fileURLToPath(new URL('..', import.meta.url))

function copyMarkdownDocsPlugin(): Plugin {
  return {
    name: 'copy-markdown-docs',
    apply: 'build',
    async closeBundle() {
      const sourceDir = path.join(repoRoot, 'docs')
      const targetDir = path.join(repoRoot, 'backend/web/dist/_docs')
      await rm(targetDir, { recursive: true, force: true })
      await copyMarkdownDocs(sourceDir, targetDir)
    },
  }
}

async function copyMarkdownDocs(sourceDir: string, targetDir: string) {
  const entries = await readdir(sourceDir, { withFileTypes: true })
  await mkdir(targetDir, { recursive: true })

  for (const entry of entries) {
    if (entry.name.startsWith('.')) {
      continue
    }

    const sourcePath = path.join(sourceDir, entry.name)
    const targetPath = path.join(targetDir, entry.name)
    if (entry.isDirectory()) {
      await copyMarkdownDocs(sourcePath, targetPath)
      continue
    }
    if (entry.isFile() && entry.name.endsWith('.md')) {
      await copyFile(sourcePath, targetPath)
    }
  }
}

export default defineConfig({
  plugins: [vue(), copyMarkdownDocsPlugin()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/openapi.json': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/openapi.yaml': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/openapi-3.0.json': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/openapi-3.0.yaml': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/docs': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/_docs': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../backend/web/dist',
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'vue-core',
              test: /node_modules[\\/](?:@vue|vue|vue-router|pinia|vue-i18n)[\\/]/,
              priority: 10,
            },
          ],
        },
      },
    },
  },
})
