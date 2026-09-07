import { usePolling } from '../../hooks/usePolling'
import { useAuth } from '../../hooks/useAuth'

export default function Header({ onMenu }: { onMenu: () => void }) {
  const { health, disconnect } = useAuth()
  // gentle health refresh
  usePolling(async () => null, 30000)

  return (
    <header className="hdr">
      <button className="menu-btn" onClick={onMenu} aria-label="Open menu">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" />
        </svg>
      </button>
      <a href="#/" className="hdr-logo">
        <img src="/logo-full.svg" alt="callhook" />
      </a>
      <div className="hdr-right">
        {health && (
          <span className={`pill ${health.dry_run ? 'dry' : 'live'}`}>
            <span className="dot" /> {health.dry_run ? 'DRY-RUN' : 'LIVE'}
          </span>
        )}
        <button className="btn sm" onClick={disconnect}>Disconnect</button>
      </div>
    </header>
  )
}
