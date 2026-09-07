import { useState } from 'react'
import { usePolling } from '../../hooks/usePolling'
import { useAuth } from '../../hooks/useAuth'
import { DocsHint } from '../domain/DocsHint'
import { DOCS } from '../../lib/docs'

// The mode badge: dry-run is amber (attention, not alarm); live is green.
// Clicking it explains what the mode means and how to change it.
export default function Header({ onMenu }: { onMenu: () => void }) {
  const { health, disconnect } = useAuth()
  // gentle health refresh
  usePolling(async () => null, 30000)
  const [modeOpen, setModeOpen] = useState(false)

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
          <button
            className={`pill ${health.dry_run ? 'dry' : 'live'}`}
            onClick={() => setModeOpen(o => !o)}
            title={health.dry_run ? 'What is dry-run mode?' : 'Live mode'}
            style={{ cursor: 'pointer', border: 'none', font: 'inherit' }}
          >
            <span className="dot" /> {health.dry_run ? 'Dry-run' : 'Live'}
          </button>
        )}
        <button className="btn sm" onClick={disconnect}>Disconnect</button>
      </div>

      {modeOpen && health && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            right: 12,
            marginTop: 6,
            width: 300,
            zIndex: 50,
          }}
          className="card"
          onClick={e => e.stopPropagation()}
        >
          {health.dry_run ? (
            <>
              <strong>Dry-run mode</strong>
              <p className="muted" style={{ fontSize: 12.5, marginTop: 8 }}>
                The full pipeline runs — events, prefetch, calls, outcomes, campaigns — but
                calls are <b>fabricated locally</b>. No phones ring, no CALL-E balance is spent.
                Perfect for demos and development.
              </p>
              <div className="section-title" style={{ margin: '10px 0 4px' }}>Why this badge?</div>
              <p className="muted" style={{ fontSize: 12.5 }}>
                So you always know whether you're watching simulated data or live calls with
                real customers.
              </p>
              <div className="section-title" style={{ margin: '10px 0 4px' }}>Go live</div>
              <p className="muted" style={{ fontSize: 12.5 }}>
                Set <span className="mono">CALLHOOK_API_KEY</span> (and{' '}
                <span className="mono">CALLHOOK_PUBLIC_URL</span>) on the server and restart.
                One env var — same binary, same code.
              </p>
              <DocsHint label="Going live: the one env var →" url={DOCS.apiKey} small />
            </>
          ) : (
            <>
              <strong>Live mode</strong>
              <p className="muted" style={{ fontSize: 12.5, marginTop: 8 }}>
                Real calls are placed through CALL-E and bill your balance. Sessions, retries
                and polite-hours deferrals behave exactly as in dry-run — the pipeline is the
                same, only the call execution is real.
              </p>
              <DocsHint label="Production checklist →" url={DOCS.production} small />
            </>
          )}
        </div>
      )}
    </header>
  )
}
