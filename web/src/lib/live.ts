// live.ts — SSE subscription to /api/stream. Session and campaign mutations
// arrive as they happen; usePolling refetches instantly on each event and
// keeps interval polling as the fallback (and for initial load).
import { useAuth } from '../hooks/useAuth'

type Listener = (event: string) => void

let source: EventSource | null = null
let listeners = new Set<Listener>()
let currentUrl = ''

export function subscribeLive(baseUrl: string, token: string) {
  const url = `${baseUrl.replace(/\/$/, '')}/api/stream${token ? `?token=${encodeURIComponent(token)}` : ''}`
  if (source && currentUrl === url) return
  if (source) source.close()

  currentUrl = url
  source = new EventSource(url)
  source.onerror = () => {
    // EventSource auto-reconnects; polling remains the safety net.
  }
  for (const name of ['session', 'campaign']) {
    source.addEventListener(name, (e) => {
      listeners.forEach((fn) => fn(name))
    })
  }
}

export function unsubscribeLive() {
  if (source) source.close()
  source = null
  currentUrl = ''
}

export function onLiveEvent(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}
