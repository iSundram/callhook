import type { ReactNode } from 'react'
import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Sidebar from './Sidebar'

export default function AppShell({ children }: { children?: ReactNode }) {
  const [drawerOpen, setDrawerOpen] = useState(false)

  return (
    <div className="app-shell">
      <Sidebar />
      <div className="app-main">
        <Header onMenu={() => setDrawerOpen(true)} />
        <div className="app-content">{children || <Outlet />}</div>
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
