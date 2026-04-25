import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../openapi/browser.yaml',
  output: 'src/api/generated',
})
