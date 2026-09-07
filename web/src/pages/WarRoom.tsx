import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { timeAgo } from '../lib/format'
import { OutcomeBadge } from '../components/domain/shared'

export default function WarRoom() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }

  const { data: metrics } = usePolling(() => api.metrics(conn), 2000)
  const { data: sessions } = usePolling(() => api.sessions(conn), 2000)
  const { data: campaigns } = usePolling(() => api.campaigns(conn), 3000)

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
          {recent.length === 0 && <div className="empty">nothing yet — fire an event or launch a campaign</div>}
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
              <Link to="/fire" className="btn">Fire an event</Link>
              <Link to="/campaigns" className="btn primary">Launch a campaign</Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
