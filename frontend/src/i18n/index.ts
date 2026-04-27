import { createI18n } from 'vue-i18n'

import { datetimeFormats, numberFormats } from './formats'
import { defaultLocale, type AppLocale } from './locales'
import { messages } from './messages'
import { loadPreferredLocale, savePreferredLocale } from './storage'

const initialLocale = loadPreferredLocale()

if (typeof document !== 'undefined') {
  document.documentElement.lang = initialLocale
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale,
  fallbackLocale: defaultLocale,
  messages,
  datetimeFormats,
  numberFormats,
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
})

export function setI18nLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  savePreferredLocale(locale)

  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale
  }
}
