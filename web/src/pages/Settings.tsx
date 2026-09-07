import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { DocsHint } from '../components/domain/DocsHint'
import { DOCS } from '../lib/docs'

const ENV_DOCS: [string, string, string][] = [
  ['CALLHOOK_API_KEY', '—', 'CALL-E key. Empty = dry-run mode.'],
  ['CALLHOOK_PUBLIC_URL', '—', 'Public URL for CALL-E webhook delivery.'],
  ['CALLHOOK_INTAKE_TOKEN', '—', 'Bearer token for all /api endpoints.'],
  ['CALLHOOK_WEBHOOK_SECRET', '—', 'X-Callhook-Secret for the webhook.'],
  ['CALLHOOK_JOURNAL', 'data/sessions.jsonl', 'Session journal (crash-safe).'],
  ['CALLHOOK_CAMPAIGN_JOURNAL', 'data/campaigns.jsonl', 'Campaign journal.'],
  ['CALLHOOK_ENFORCE_WINDOWS', 'true', 'Defer calls outside 9–20h local, weekdays.'],
  ['CALLHOOK_RETRY_DELAY', '2h', 'Redial delay after no_answer.'],
  ['CALLHOOK_MAX_CONCURRENT', '3', 'Max in-flight calls per wave.'],
]

export default function Settings() {
  const { health, baseUrl, token, disconnect } = useAuth()
  usePolling(async () => null, 60000) // keep mounted timer

  return (
    <div>
      <div className="page-header">
        <h1>Settings</h1>
        <p>Connection and server configuration snapshot.</p>
      </div>

      <div className="grid2">
        <div className="card">
          <strong>Connection</strong>
          <div style={{ marginTop: 12 }}>
            {[
              ['Server', baseUrl || '—'],
              ['Token', token ? '••••••••' : '(none)'],
              ['Mode', health?.dry_run ? 'DRY-RUN (no real calls)' : 'LIVE'],
              ['Windows', health?.windows_enforced ? 'enforced (9–20h local, weekdays)' : 'disabled'],
              ['Intake auth', health?.auth_intake ? 'required' : 'open'],
              ['Webhook auth', health?.auth_webhook ? 'required' : 'open'],
            ].map(([k, v]) => (
              <div key={k as string} className="spread" style={{ padding: '8px 0', borderBottom: '1px solid var(--border-subtle)' }}>
                <span className="muted">{k}</span>
                <span className="mono" style={{ fontSize: 12.5 }}>{v}</span>
              </div>
            ))}
          </div>
          <div style={{ marginTop: 16 }}>
            <button className="btn danger" onClick={disconnect}>Disconnect</button>
          </div>
          {health?.dry_run ? (
            <div style={{ marginTop: 14 }}>
              <p className="faint" style={{ fontSize: 12 }}>
                Dry-run: the full pipeline runs with fabricated calls — set CALLHOOK_API_KEY to go live.
              </p>
              <DocsHint label="Going live: the one env var →" url={DOCS.apiKey} small />
            </div>
          ) : (
            health && <DocsHint label="Live mode — production checklist →" url={DOCS.production} small />
          )}
          {health && (!health.auth_intake || !health.auth_webhook) && (
            <DocsHint label="Endpoints are unauthenticated — lock it down →" url={DOCS.auth} small />
          )}
        </div>

        <div className="card">
          <strong>Server environment reference</strong>
          <p className="faint" style={{ fontSize: 12, margin: '6px 0 10px' }}>Set on the server — shown here for convenience.</p>
          {ENV_DOCS.map(([k, def, desc]) => (
            <div key={k} style={{ padding: '7px 0', borderBottom: '1px solid var(--border-subtle)' }}>
              <div className="mono" style={{ fontSize: 12.5, color: 'var(--brand-silver)' }}>{k}</div>
              <div className="muted" style={{ fontSize: 12 }}>{desc} <span className="faint">(default: {def})</span></div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
