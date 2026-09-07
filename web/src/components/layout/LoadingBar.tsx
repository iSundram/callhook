import { useSyncExternalStore } from 'react'
import { subscribeLoading, isLoading } from '../../lib/loading'

// The global progress bar: sits at the very top (above the header), runs
// while any API request is in flight. The app dims beneath it.
export default function LoadingBar() {
  const loading = useSyncExternalStore(subscribeLoading, isLoading)

  return (
    <div className={`load-bar ${loading ? 'on' : ''}`} aria-hidden={!loading}>
      <div className="load-bar-fill" />
    </div>
  )
}
