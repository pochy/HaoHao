export const datetimeFormats = {
  en: {
    short: {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
    },
    long: {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    },
  },
  ja: {
    short: {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    },
    long: {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    },
  },
} as const

export const numberFormats = {
  en: {
    integer: {
      maximumFractionDigits: 0,
    },
    percent: {
      style: 'percent',
      maximumFractionDigits: 1,
    },
  },
  ja: {
    integer: {
      maximumFractionDigits: 0,
    },
    percent: {
      style: 'percent',
      maximumFractionDigits: 1,
    },
  },
} as const
