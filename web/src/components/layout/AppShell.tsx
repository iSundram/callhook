import type { ReactNode } from 'react'
import { useState, useSyncExternalStore } from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Sidebar from './Sidebar'
import LoadingBar from './LoadingBar'
import { subscribeLoading, isLoading } from '../../lib/loading'

export default function AppShell({ children }: { children?: ReactNode }) {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const loading = useSyncExternalStore(subscribeLoading, isLoading)

  return (
    <div className="app-shell">
      <LoadingBar />
      <Sidebar />
      <div className="app-main">
        <Header onMenu={() => setDrawerOpen(true)} />
        {/* Dim everything while data loads — content stays visible, just recedes. */}
        <div className={`app-content ${loading ? 'dimmed' : ''}`}>{children || <Outlet />}</div>
      </div>

      {drawerOpen && (
        <>
          <div className="drawer-overlay" onClick={() => setDrawerOpen(false)} />
          <div className="drawer">
            <div className="drawer-head">
              <img src="/logo-full.svg" alt="callhook" className="drawer-logo" />
              <button className="btn sm" onClick={() => setDrawerOpen(false)}>✕</button>
            </div>
            <div onClick={() => setDrawerOpen(false)}>
              <Sidebar />
            </div>
          </div>
        </>
      )}
    </div>
  )
}
