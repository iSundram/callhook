import type { ReactNode } from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Sidebar from './Sidebar'

export default function AppShell({ children }: { children?: ReactNode }) {
  return (
    <div className="app-shell">
      <Sidebar />
      <div className="app-main">
        <Header />
        <div className="app-content">{children || <Outlet />}</div>
      </div>
    </div>
  )
}
