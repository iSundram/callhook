import { useEffect, useRef, useState } from 'react'
import { onLiveEvent, isLiveConnected } from '../lib/live'

// usePolling — fetch on an interval with the previous value kept during
// refetches (no flicker). The workhorse behind every live view.
//
// staleFor: when a refetch takes longer than the grace period (400ms),
// `stale` flips true — views swap the data area for shimmer ghosts.
// Fast refreshes (the common case) never show it.
// Grace before stale data swaps to shimmer ghosts. Short enough that
// plainly-slow fetches trigger it; fast ones still slip through.
export const STALE_GRACE_MS = 150

// Once skeletons are visible (first load or stale), keep them on screen at
// least this long — a 100ms flash reads as a glitch, not as loading.
const MIN_SKELETON_MS = 600

// When the SSE stream is open, mutations arrive pushed and the poll is only
// a heartbeat; if it drops, return to the tight fallback interval.
const HEARTBEAT_MS = 20_000

const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms))

export function usePolling<T>(fn: () => Promise<T>, intervalMs: number, enabled = true) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [stale, setStale] = useState(false)
  const fnRef = useRef(fn)
  fnRef.current = fn
  const dataRef = useRef<T | null>(null)
  dataRef.current = data

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
      const firstLoad = dataRef.current === null
      const startedAt = Date.now()
      let ghostShownAt = 0
      graceTimer = setTimeout(() => {
        if (alive) {
          ghostShownAt = Date.now()
          setStale(true) // slow fetch → ghosts replace stale data
        }
      }, STALE_GRACE_MS)
      try {
        const v = await fnRef.current()
        if (!alive) return
        // Minimum skeleton visibility: first loads and ghost swaps hold at
        // least MIN_SKELETON_MS so the shimmer is perceivable, never a flash.
        const shownAt = ghostShownAt || (firstLoad ? startedAt : 0)
        if (shownAt) {
          const elapsed = Date.now() - shownAt
          if (elapsed < MIN_SKELETON_MS) await sleep(MIN_SKELETON_MS - elapsed)
        }
        if (!alive) return
        setData(v)
        setError(null)
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
