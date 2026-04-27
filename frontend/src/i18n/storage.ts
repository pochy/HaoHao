import { defaultLocale, isAppLocale, type AppLocale } from './locales'

const localeStorageKey = 'haohao.locale'

export function loadPreferredLocale(): AppLocale {
  if (typeof window === 'undefined') {
    return defaultLocale
  }

  const savedLocale = window.localStorage.getItem(localeStorageKey)
  return isAppLocale(savedLocale) ? savedLocale : defaultLocale
}

export function savePreferredLocale(locale: AppLocale) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(localeStorageKey, locale)
}
