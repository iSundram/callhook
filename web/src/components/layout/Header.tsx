import { usePolling } from '../../hooks/usePolling'
import { useAuth } from '../../hooks/useAuth'

export default function Header() {
  const { health, disconnect } = useAuth()
  // gentle health refresh
  usePolling(async () => null, 30000)

  return (
    <header className="hdr">
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
