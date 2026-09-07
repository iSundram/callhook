import { useEffect, useRef, useState } from 'react'
import { onLiveEvent } from '../lib/live'

// usePolling — fetch on an interval with the previous value kept during
// refetches (no flicker). The workhorse behind every live view.
export function usePolling<T>(fn: () => Promise<T>, intervalMs: number, enabled = true) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const fnRef = useRef(fn)
  fnRef.current = fn

  // Live events (SSE) trigger an immediate refetch; the interval is the
  // fallback and the initial load.
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

    async function tick() {
      try {
        const v = await fnRef.current()
        if (alive) {
          setData(v)
          setError(null)
        }
      } catch (e: any) {
        if (alive) setError(e.message || 'request failed')
      } finally {
        if (alive) timer = setTimeout(tick, intervalMs)
      }
    }
    tick()
    return () => {
      alive = false
      clearTimeout(timer)
    }
  }, [intervalMs, enabled])

  return { data, error, setData }
}
