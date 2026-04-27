export const supportedLocales = ['en', 'ja'] as const

export type AppLocale = typeof supportedLocales[number]

export const defaultLocale: AppLocale = 'en'

export function isAppLocale(value: string | null | undefined): value is AppLocale {
  return supportedLocales.includes(value as AppLocale)
}
