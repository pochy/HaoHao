<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { checkDocsAccess } from '../api/docs'

const { t } = useI18n()
const checking = ref(false)
const errorMessage = ref('')

async function openDocs() {
  checking.value = true
  errorMessage.value = ''

  try {
    await checkDocsAccess()
    window.open('/docs/openapi', '_blank', 'noreferrer')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('docs.unavailable')
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <div class="docs-link-wrapper">
    <button class="secondary-button" :disabled="checking" type="button" @click="openDocs">
      {{ checking ? t('docs.checking') : t('docs.open') }}
    </button>
    <p v-if="errorMessage" class="error-message">
      {{ errorMessage }}
    </p>
  </div>
</template>

<style scoped>
.docs-link-wrapper {
  display: inline-grid;
  gap: 8px;
}
</style>
