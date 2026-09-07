// Global loading state: a tiny store so any data fetch can dim the app and
// drive the top progress bar. First load shows skeletons; in-flight refreshes
// just pulse the bar (content stays, no flicker).

type Listener = () => void

let inflight = 0
const listeners = new Set<Listener>()

function notify() {
  listeners.forEach(l => l())
}

/** Wrap any async fetch: registers global loading while it runs. */
export async function track<T>(fn: () => Promise<T>): Promise<T> {
  inflight++
  notify()
  try {
    return await fn()
  } finally {
    inflight--
    notify()
  }
}

/** True while any tracked fetch is in flight. */
export function isLoading(): boolean {
  return inflight > 0
}

export function subscribeLoading(l: Listener): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}
