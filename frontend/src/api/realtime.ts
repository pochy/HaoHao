export type RealtimeTransportStatus = 'idle' | 'connecting' | 'connected' | 'polling' | 'error'

export type RealtimeEvent = {
  cursor: string
  publicId?: string
  tenantId?: number
  type: string
  resourceType?: string
  resourcePublicId?: string
  payload?: Record<string, unknown>
  createdAt?: string
}

type RealtimePollResponse = {
  items?: RealtimeEvent[] | null
  cursor?: string
}

type RealtimeConnectionOptions = {
  cursor?: string
  storageKey?: string
  onEvent: (event: RealtimeEvent) => void
  onCursor?: (cursor: string) => void
  onStatus?: (status: RealtimeTransportStatus) => void
}

type RealtimeConnection = {
  close: () => void
}

const REALTIME_EVENT_TYPES = [
  'realtime.ready',
  'notification.created',
  'notification.read',
  'job.updated',
]

export function connectRealtime(options: RealtimeConnectionOptions): RealtimeConnection {
  let closed = false
  let source: EventSource | null = null
  let retryTimer: number | undefined
  let pollController: AbortController | null = null
  const storageKey = options.storageKey ?? 'haohao.realtime.cursor'
  let cursor = options.cursor ?? window.localStorage.getItem(storageKey) ?? ''

  const setStatus = (status: RealtimeTransportStatus) => options.onStatus?.(status)
  const setCursor = (next: string) => {
    if (!next) {
      return
    }
    cursor = next
    window.localStorage.setItem(storageKey, next)
    options.onCursor?.(next)
  }

  const handleEvent = (event: RealtimeEvent) => {
    setCursor(event.cursor)
    if (event.type !== 'realtime.ready') {
      options.onEvent(event)
    }
  }

  const startPolling = () => {
    if (closed) {
      return
    }
    source?.close()
    source = null
    setStatus('polling')
    void pollLoop()
  }

  const schedulePolling = () => {
    if (closed || retryTimer !== undefined) {
      return
    }
    retryTimer = window.setTimeout(() => {
      retryTimer = undefined
      startPolling()
    }, 1000)
  }

  const pollLoop = async () => {
    while (!closed) {
      pollController?.abort()
      pollController = new AbortController()
      try {
        const query = new URLSearchParams()
        if (cursor) {
          query.set('cursor', cursor)
        }
        query.set('timeoutSeconds', '25')
        const response = await fetch(`/api/v1/realtime/events/poll?${query.toString()}`, {
          credentials: 'include',
          headers: { Accept: 'application/json' },
          signal: pollController.signal,
        })
        if (response.status === 204) {
          continue
        }
        if (!response.ok) {
          throw new Error(`Realtime polling failed (${response.status})`)
        }
        const body = await response.json() as RealtimePollResponse
        if (body.cursor) {
          setCursor(body.cursor)
        }
        for (const item of body.items ?? []) {
          handleEvent(item)
        }
      } catch (error) {
        if (closed || (error instanceof DOMException && error.name === 'AbortError')) {
          return
        }
        setStatus('error')
        await sleep(2000)
        setStatus('polling')
      }
    }
  }

  if ('EventSource' in window) {
    setStatus('connecting')
    const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''
    source = new EventSource(`/api/v1/realtime/events${query}`, { withCredentials: true })
    source.onopen = () => setStatus('connected')
    source.onerror = () => {
      setStatus('error')
      schedulePolling()
    }
    for (const type of REALTIME_EVENT_TYPES) {
      source.addEventListener(type, (message) => {
        try {
          const data = JSON.parse((message as MessageEvent).data) as RealtimeEvent
          if (!data.type) {
            data.type = type
          }
          if (!data.cursor && (message as MessageEvent).lastEventId) {
            data.cursor = (message as MessageEvent).lastEventId
          }
          handleEvent(data)
        } catch {
          setStatus('error')
        }
      })
    }
  } else {
    startPolling()
  }

  return {
    close() {
      closed = true
      if (retryTimer !== undefined) {
        window.clearTimeout(retryTimer)
      }
      source?.close()
      pollController?.abort()
      setStatus('idle')
    },
  }
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}
