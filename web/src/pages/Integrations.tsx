import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { DocsHint } from '../components/domain/DocsHint'
import { DOCS } from '../lib/docs'

// The live integrations catalog, served by the backend
// (GET /api/integrations) — platform, route, env vars, signature scheme,
// and whether each one's secret is configured on the server.
export default function Integrations() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const { data, stale } = usePolling(() => api.integrations(conn), 15000)

  const list: any[] = data?.integrations || []
  const configured = list.filter(i => i.env_configured).length

  return (
    <div>
      <div className="page-header">
        <h1>Integrations</h1>
        <p>Point any platform's webhook at callhook — one endpoint, verified signatures, intelligent calls out.</p>
      </div>

      <div className="stat-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
        <div className="card stat">
          <div className="label">Platforms</div>
          <div className="value">{list.length}</div>
          <div className="sub">webhook adapters, built in</div>
        </div>
        <div className="card stat ok">
          <div className="label">Configured</div>
          <div className="value">{configured}</div>
          <div className="sub">secrets present on this server</div>
        </div>
        <div className="card stat">
          <div className="label">Universal</div>
          <div className="value">+1</div>
          <div className="sub">generic HTTP adapter for anything else</div>
        </div>
      </div>

      <div className="card" style={{ marginTop: 18 }}>
        <div className="spread" style={{ marginBottom: 12 }}>
          <strong>Adapter catalog</strong>
          <span className="faint" style={{ fontSize: 12 }}>
            signature schemes verified against each platform's official docs
          </span>
        </div>
        {(data === null || stale) && (
          <div aria-busy="true">
            {[0, 1, 2, 3, 4, 5].map(i => (
              <div key={i} style={{ display: 'flex', gap: 10, alignItems: 'center', padding: '12px 0', borderBottom: '1px solid var(--border-subtle)' }}>
                <div className="skeleton" style={{ height: 12, width: '14%' }} />
                <div className="skeleton" style={{ height: 12, width: '24%' }} />
                <div className="skeleton" style={{ height: 12, width: '30%', marginLeft: 'auto' }} />
              </div>
            ))}
          </div>
        )}
        {data !== null && !stale && list.length === 0 && <div className="empty">catalog unavailable — is the server up to date?</div>}
        {data !== null && !stale && list.map((i: any) => (
          <div key={i.platform} style={{ padding: '12px 0', borderBottom: '1px solid var(--border-subtle)' }}>
            <div className="spread" style={{ alignItems: 'baseline' }}>
              <div className="row">
                <span className="mono" style={{ fontWeight: 700, fontSize: 13.5 }}>{i.platform}</span>
                <span className={`pill ${i.env_configured ? 'ok' : 'muted'}`}>
                  {i.env_configured ? 'ready' : 'not configured'}
                </span>
              </div>
              <span className="mono faint" style={{ fontSize: 12 }}>{i.route}</span>
            </div>
            <div className="muted" style={{ fontSize: 12.5, margin: '6px 0' }}>{i.verified}</div>
            <div className="row" style={{ gap: 10, alignItems: 'baseline' }}>
              <span className="faint" style={{ fontSize: 11.5 }}>
                {i.env_configured ? 'env: ' : i.hint}
              </span>
              {i.env_configured && (
                <span className="mono faint" style={{ fontSize: 11.5 }}>
                  {i.env_vars.join(', ')}
                </span>
              )}
              <a
                href={i.doc_url}
                target="_blank"
                rel="noopener"
                className="faint"
                style={{ fontSize: 11.5, marginLeft: 'auto' }}
              >
                docs ↗
              </a>
            </div>
          </div>
        ))}
      </div>

      <div className="card" style={{ marginTop: 18 }}>
        <strong>Send a signed test webhook</strong>
        <p className="muted" style={{ fontSize: 13, marginTop: 8 }}>
          <span className="mono">callhookctl test-webhook stripe</span> signs a fixture with the
          platform's real scheme and posts it to your server — the same code path a live Stripe
          delivery takes. See <span className="mono">integrations/&lt;platform&gt;/README.md</span> in
          the repo for each platform's dashboard setup steps.
        </p>
        <DocsHint label="Webhook 401 or invalid signature? Troubleshooting →" url={DOCS.tsWebhook401} small />
      </div>

      <div className="card" style={{ marginTop: 18 }}>
        <strong>Any other platform?</strong>
        <p className="muted" style={{ fontSize: 13, marginTop: 8 }}>
          Use the <span className="mono">generic</span> adapter: POST callhook event JSON to{' '}
          <span className="mono">/integrations/generic/webhook</span> with a bearer token or{' '}
          <span className="mono">X-Callhook-Secret</span> header. n8n, Zapier, LangChain, OpenAI
          function-calling, Airtable and Google Forms recipes ship in{' '}
          <span className="mono">plugins/</span> and <span className="mono">integrations/</span>.
        </p>
        <DocsHint label="Full integration docs →" url={DOCS.nativeAdapters} small />
      </div>
    </div>
  )
}
