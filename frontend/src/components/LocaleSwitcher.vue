<script setup lang="ts">
import { Languages } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { setI18nLocale } from '../i18n'
import { isAppLocale, supportedLocales } from '../i18n/locales'

const { locale, t } = useI18n({ useScope: 'global' })

function changeLocale(event: Event) {
  const nextLocale = (event.target as HTMLSelectElement).value
  if (isAppLocale(nextLocale)) {
    setI18nLocale(nextLocale)
  }
}
</script>

<template>
  <label class="locale-switcher">
    <Languages :size="16" stroke-width="1.8" aria-hidden="true" />
    <span class="sr-only">{{ t('settings.locale') }}</span>
    <select
      data-testid="locale-switcher"
      class="locale-switcher-select"
      :aria-label="t('settings.locale')"
      :value="locale"
      @change="changeLocale"
    >
      <option v-for="item in supportedLocales" :key="item" :value="item">
        {{ t(`settings.localeNames.${item}`) }}
      </option>
    </select>
  </label>
</template>
