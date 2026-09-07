import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { timeAgo } from '../lib/format'
import { useState } from 'react'
import { OutcomeBadge } from '../components/domain/shared'
import { DocsHint } from '../components/domain/DocsHint'
import { DOCS } from '../lib/docs'
import { EVENT_TYPES } from '../lib/events'

export default function WarRoom() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }

  const { data: metrics, error: metricsErr } = usePolling(() => api.metrics(conn), 2000)
  const { data: sessions } = usePolling(() => api.sessions(conn), 2000)
  const { data: campaigns } = usePolling(() => api.campaigns(conn), 3000)

  const [demoBusy, setDemoBusy] = useState(false)

  // First-run demo: three events + one campaign — turns an empty install
  // into a living war room in one click.
  async function runDemo() {
    setDemoBusy(true)
    try {
      // Fire one event of each type across the demo customers.
      const customers = ['cus_1001', 'cus_1002', 'cus_1003']
      for (let i = 0; i < EVENT_TYPES.length; i++) {
        await api.fire(conn, {
          id: `demo_evt_${Date.now()}_${i}`,
          type: EVENT_TYPES[i].type,
          customer_id: customers[i % customers.length],
          payload: EVENT_TYPES[i].demo,
        })
      }
      await api.createCampaign(conn, {
        name: 'Demo campaign — September collections',
        event_type: 'invoice.due',
        goal: { type: 'count', target: 5, success_outcomes: ['payment_promised'] },
        audience_source: 'all_overdue',
        waves: { size: 3, delay: '15s', max_waves: 5 },
        budget: { max_calls: 15 },
      })
    } finally {
      setDemoBusy(false)
    }
  }

  const recent = (sessions || []).slice(0, 8)
  const running = (campaigns || []).filter(c => c.status === 'running')
  const inFlight = metrics?.by_status?.in_progress || 0
  const outcomes: [string, number][] = metrics
    ? (Object.entries(metrics.by_outcome) as [string, number][]).sort((a, b) => b[1] - a[1]).slice(0, 4)
    : []

  return (
    <div>
      <div className="page-header">
        <h1>War Room</h1>
        <p>Everything happening on your voice channel, right now.</p>
      </div>

      {metricsErr && (
        <div className="card" style={{ marginBottom: 16, borderColor: 'rgba(252,165,165,.3)' }}>
          <div className="pill err" style={{ display: 'inline-block' }}>{metricsErr}</div>
          <p className="muted" style={{ fontSize: 12.5, marginTop: 8 }}>
            Live stats can't load — the session token was rejected or the server is unreachable.
            Click <b>Disconnect</b>, then reconnect with the server's <span className="mono">CALLHOOK_INTAKE_TOKEN</span>.
          </p>
        </div>
      )}

      <div className="stat-grid">
        <div className="card stat">
          <div className="label">Sessions</div>
          <div className="value">{metrics?.sessions ?? '—'}</div>
        </div>
        <div className={`card stat ${inFlight ? 'warn' : ''}`}>
          <div className="label">In flight</div>
          <div className="value">{inFlight}</div>
          <div className="sub">{metrics?.retries_armed ?? 0} armed retries</div>
        </div>
        <div className="card stat ok">
          <div className="label">Completed</div>
          <div className="value">{metrics?.by_status?.completed ?? '—'}</div>
        </div>
        <div className={`card stat ${metrics?.by_status?.failed ? 'err' : ''}`}>
          <div className="label">Failed</div>
          <div className="value">{metrics?.by_status?.failed ?? '—'}</div>
        </div>
        <div className={`card stat ${running.length ? 'warn' : ''}`}>
          <div className="label">Campaigns</div>
          <div className="value">{running.length}</div>
          <div className="sub">{campaigns?.length ?? 0} total</div>
        </div>
      </div>

      <div className="grid2">
        <div className="card">
          <div className="spread" style={{ marginBottom: 10 }}>
            <strong>Live activity</strong>
            <Link to="/sessions" className="faint" style={{ fontSize: 12 }}>all sessions →</Link>
          </div>
          {recent.length === 0 && (
            <div className="empty">
              nothing yet<br />
              <button className="btn primary" style={{ marginTop: 14 }} disabled={demoBusy} onClick={runDemo}>
                {demoBusy ? 'Running…' : 'Run the demo'}
              </button>
              <p className="faint" style={{ fontSize: 11.5, marginTop: 8 }}>fires one event of each type + launches a campaign (dry-run, free)</p>
            </div>
          )}
          {recent.map(s => (
            <Link key={s.id} to={`/sessions/${s.id}`} className="row" style={{ padding: '8px 0', borderBottom: '1px solid var(--border-subtle)', justifyContent: 'space-between' }}>
              <span className="mono">{s.customer || s.customer_id}</span>
              <span className="pill muted">{s.event_type}</span>
              <OutcomeBadge outcome={s.outcome?.outcome} />
              <span className="faint" style={{ fontSize: 11.5 }}>{timeAgo(s.updated_at)}</span>
            </Link>
          ))}
        </div>

        <div>
          <div className="card" style={{ marginBottom: 16 }}>
            <div className="spread" style={{ marginBottom: 10 }}>
              <strong>Outcomes</strong>
              <Link to="/campaigns" className="faint" style={{ fontSize: 12 }}>campaigns →</Link>
            </div>
            {outcomes.length === 0 && <div className="empty">no outcomes yet</div>}
            {outcomes.map(([name, count]) => (
              <div key={name} className="row" style={{ justifyContent: 'space-between', padding: '7px 0', borderBottom: '1px solid var(--border-subtle)' }}>
                <OutcomeBadge outcome={name} />
                <span className="mono" style={{ fontWeight: 700 }}>{count}</span>
              </div>
            ))}
          </div>
          <div className="card">
            <strong>Quick actions</strong>
            <div className="row" style={{ marginTop: 12 }}>
              <button className="btn primary" disabled={demoBusy} onClick={runDemo}>
                {demoBusy ? 'Running…' : 'Run the demo'}
              </button>
              <Link to="/fire" className="btn">Fire an event</Link>
              <Link to="/campaigns" className="btn">Launch a campaign</Link>
            </div>
            <div style={{ marginTop: 12 }}>
              <div style={{ padding: '6px 0', borderBottom: '1px solid var(--border-subtle)', fontSize: 12.5 }}>
                <b>Run the demo</b> — all 8 event types + a campaign. Everything appears live below.
              </div>
              <div style={{ padding: '6px 0', borderBottom: '1px solid var(--border-subtle)', fontSize: 12.5 }}>
                <b>Fire an event</b> — push a single event through the pipeline by hand, watch it end-to-end.
              </div>
              <div style={{ padding: '6px 0', fontSize: 12.5 }}>
                <b>Launch a campaign</b> — declare a goal (e.g. "5 payment promises"), callhook runs the waves.
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
