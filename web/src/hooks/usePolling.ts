import { useEffect, useRef, useState } from 'react'
import { onLiveEvent, isLiveConnected } from '../lib/live'

// usePolling — fetch on an interval with the previous value kept during
// refetches (no flicker). The workhorse behind every live view.
//
// staleFor: when a refetch takes longer than the grace period (400ms),
// `stale` flips true — views swap the data area for shimmer ghosts.
// Fast refreshes (the common case) never show it.
export const STALE_GRACE_MS = 400

// When the SSE stream is open, mutations arrive pushed and the poll is only
// a heartbeat; if it drops, return to the tight fallback interval.
const HEARTBEAT_MS = 20_000

export function usePolling<T>(fn: () => Promise<T>, intervalMs: number, enabled = true) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [stale, setStale] = useState(false)
  const fnRef = useRef(fn)
  fnRef.current = fn

  useEffect(() => {
    if (!enabled) return
    const off = onLiveEvent(() => {
      fnRef.current().then(setData).catch(() => {})
    })
    return off
  }, [enabled])

  useEffect(() => {
    if (!enabled) return
    let alive = true
    let timer: ReturnType<typeof setTimeout>
    let graceTimer: ReturnType<typeof setTimeout> | undefined

    async function tick() {
      graceTimer = setTimeout(() => {
        if (alive) setStale(true) // slow fetch → ghosts replace stale data
      }, STALE_GRACE_MS)
      try {
        const v = await fnRef.current()
        if (alive) {
          setData(v)
          setError(null)
        }
      } catch (e: any) {
        if (!alive) return
        const msg = e.message || 'request failed'
        setError(msg)
        // Auth lost (token rejected/rotated server-side): drop the saved
        // session so the Connect screen reappears with the reason, instead
        // of every view silently showing placeholders.
        if (msg.includes('bearer') || msg.includes('401') || msg.includes('Unauthorized')) {
          try {
            localStorage.removeItem('ch_baseurl')
            localStorage.removeItem('ch_token')
          } catch { /* storage restrictions */ }
          window.location.hash = ''
          window.location.reload()
        }
      } finally {
        if (graceTimer) clearTimeout(graceTimer)
        if (alive) {
          setStale(false)
          const next = isLiveConnected() ? Math.max(intervalMs, HEARTBEAT_MS) : intervalMs
          timer = setTimeout(tick, next)
        }
      }
    }
    tick()
    return () => {
      alive = false
      clearTimeout(timer)
      if (graceTimer) clearTimeout(graceTimer)
    }
  }, [intervalMs, enabled])

  return { data, error, stale, setData }
}
