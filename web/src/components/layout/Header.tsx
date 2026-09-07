import { usePolling } from '../../hooks/usePolling'
import { useAuth } from '../../hooks/useAuth'

export default function Header() {
  const { health, baseUrl, disconnect } = useAuth()
  // gentle health refresh
  usePolling(async () => null, 30000)

  const live = health ? !health.dry_run : false
  return (
    <header className="hdr">
      <div className="hdr-logo">
        <img src="/icon-tight.svg" alt="callhook" />
        <span>callhook</span>
      </div>
      <div className="hdr-right">
        {health && (
          <>
            <span className={`pill ${health.dry_run ? 'dry' : 'live'}`}>
              <span className="dot" /> {health.dry_run ? 'DRY-RUN' : 'LIVE'}
            </span>
            <span className="pill muted">{health.windows_enforced ? 'windows on' : 'windows off'}</span>
          </>
        )}
        <span className="pill muted mono">{baseUrl.replace(/^https?:\/\//, '') || 'local'}</span>
        <button className="btn sm" onClick={disconnect}>Disconnect</button>
      </div>
    </header>
  )
}
