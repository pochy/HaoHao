<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(defineProps<{
  title?: string
  message?: string
  roleLabel?: string
}>(), {
  title: undefined,
  message: '',
  roleLabel: 'machine_client_admin',
})

const { t } = useI18n()
</script>

<template>
  <section class="panel stack">
    <span class="status-pill danger">{{ t('access.forbidden') }}</span>
    <h2>{{ title ?? t('access.defaultTitle') }}</h2>
    <p v-if="message">
      {{ message }}
    </p>
    <p v-else>
      {{ t('access.defaultMessage', { role: roleLabel }) }}
    </p>
  </section>
</template>
