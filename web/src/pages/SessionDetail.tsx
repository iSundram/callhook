import { Link, useParams } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { timeAgo } from '../lib/format'
import { TranscriptView, ActionTimeline, OutcomeBadge, StatusPill } from '../components/domain/shared'

export default function SessionDetail() {
  const { id } = useParams()
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const { data: sessions, error } = usePolling(() => api.sessions(conn), 2000)

  if (sessions === null && !error) {
    return (
      <div aria-busy="true">
        <div className="page-header">
          <div className="skeleton" style={{ height: 28, width: '30%', marginBottom: 8 }} />
          <div className="skeleton" style={{ height: 16, width: '20%' }} />
        </div>
        <div className="stat-grid">
          {[0, 1, 2].map(i => (
            <div key={i} className="card stat">
              <div className="skeleton" style={{ height: 12, width: '40%', marginBottom: 10 }} />
              <div className="skeleton" style={{ height: 22, width: '30%' }} />
            </div>
          ))}
        </div>
        <div className="grid2" style={{ marginTop: 16 }}>
          <div className="card">
            <div className="skeleton" style={{ height: 16, width: '30%', marginBottom: 14 }} />
            <div className="skeleton" style={{ height: 120, width: '100%' }} />
          </div>
          <div className="card">
            <div className="skeleton" style={{ height: 16, width: '30%', marginBottom: 14 }} />
            <div className="skeleton" style={{ height: 120, width: '100%' }} />
          </div>
        </div>
      </div>
    )
  }

  const s = (sessions || []).find(x => x.id === id)

  if (!s) {
    return (
      <div>
        <div className="empty">
          session not found (expired after restart?)<br />
          <Link to="/sessions" className="btn" style={{ marginTop: 14, display: 'inline-block' }}>← back to sessions</Link>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div className="row" style={{ justifyContent: 'space-between' }}>
          <div>
            <h1>{s.customer || s.customer_id}</h1>
            <p className="mono">{s.id} · {s.phone}</p>
          </div>
          <div className="row">
            <span className="pill muted">{s.event_type}</span>
            <StatusPill status={s.call_status} />
            <OutcomeBadge outcome={s.outcome?.outcome} />
          </div>
        </div>
      </div>

      <div className="stat-grid">
        <div className="card stat">
          <div className="label">Call</div>
          <div className="value" style={{ fontSize: 13 }}>{s.call_id || '—'}</div>
        </div>
        <div className="card stat">
          <div className="label">Attempt</div>
          <div className="value">{s.retry_count + 1}</div>
        </div>
        <div className="card stat">
          <div className="label">Confidence</div>
          <div className="value" style={{ fontSize: 16 }}>{s.outcome?.summary ? 'extracted' : '—'}</div>
          <div className="sub">{timeAgo(s.updated_at)}</div>
        </div>
      </div>

      <div className="grid2">
        <div>
          <div className="section-title">Transcript</div>
          <TranscriptView turns={s.transcript} />

          <div className="section-title">Outcome</div>
          {s.outcome ? (
            <pre className="code">{JSON.stringify(s.outcome, null, 2)}</pre>
          ) : (
            <div className="empty">no outcome yet</div>
          )}

          <div className="section-title">Task sent to CALL-E</div>
          <pre className="code">{s.task || '—'}</pre>
        </div>
        <div>
          <div className="section-title">Audit trail</div>
          <ActionTimeline actions={s.actions} />
          {s.next_retry_at && (
            <div className="card" style={{ marginTop: 16 }}>
              <strong>Waiting</strong>
              <p className="muted" style={{ fontSize: 12.5, marginTop: 6 }}>
                Next trigger: <span className="pill warn">{s.next_retry_kind}</span> at {new Date(s.next_retry_at).toLocaleTimeString()}
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
