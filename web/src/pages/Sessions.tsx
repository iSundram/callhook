import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { timeAgo } from '../lib/format'
import { OutcomeBadge, StatusPill } from '../components/domain/shared'

export default function Sessions() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const { data: sessions } = usePolling(() => api.sessions(conn), 2000)
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')

  const filtered = (sessions || []).filter(s => {
    if (status && (s.call_status || 'intake') !== status) return false
    if (!q) return true
    const hay = `${s.customer} ${s.customer_id} ${s.event_type} ${s.phone} ${s.id} ${s.outcome?.outcome || ''}`.toLowerCase()
    return hay.includes(q.toLowerCase())
  })

  return (
    <div>
      <div className="page-header">
        <h1>Sessions</h1>
        <p>Every event's journey: intake → call → outcome → actions.</p>
      </div>

      <div className="row" style={{ marginBottom: 14 }}>
        <input className="input" style={{ maxWidth: 320 }} placeholder="Search customer, phone, outcome…" value={q} onChange={e => setQ(e.target.value)} />
        <select className="select" style={{ maxWidth: 200 }} value={status} onChange={e => setStatus(e.target.value)}>
          <option value="">all statuses</option>
          {['intake', 'queued', 'in_progress', 'completed', 'failed', 'canceled'].map(s => <option key={s} value={s}>{s}</option>)}
        </select>
        <span className="faint" style={{ fontSize: 12 }}>{filtered.length} session(s)</span>
      </div>

      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        {filtered.length === 0 ? (
          <div className="empty">no sessions match</div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Customer</th><th>Event</th><th>Status</th><th>Outcome</th>
                <th>Phone</th><th>Attempt</th><th>Updated</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(s => (
                <tr key={s.id} onClick={() => window.location.href = `#/sessions/${s.id}`}>
                  <td>
                    <Link to={`/sessions/${s.id}`} onClick={e => e.stopPropagation()}>
                      <strong>{s.customer || s.customer_id}</strong>
                    </Link>
                    {s.campaign_id && <span className="pill muted" style={{ marginLeft: 8, fontSize: 10 }}>campaign</span>}
                  </td>
                  <td><span className="pill muted">{s.event_type}</span></td>
                  <td><StatusPill status={s.call_status} /></td>
                  <td><OutcomeBadge outcome={s.outcome?.outcome} /></td>
                  <td className="mono">{s.phone || '—'}</td>
                  <td className="mono">{s.retry_count ? `${s.retry_count + 1}` : '1'}</td>
                  <td className="faint">{timeAgo(s.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
