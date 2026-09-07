import { Link, useParams } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import { AudienceGrid, WaveTimeline } from '../components/domain/shared'

const STATUS_TONE: Record<string, string> = {
  running: 'warn', goal_met: 'ok', exhausted: 'muted', budget_exhausted: 'err', stopped: 'muted',
}

export default function CampaignDetail() {
  const { id } = useParams()
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const { data: c } = usePolling(() => api.campaign(conn, id!), 2000)

  if (!c) return <div className="empty">campaign not found</div>

  const target = c.goal.type === 'count' ? c.goal.target : c.audience.length
  const pct = target ? Math.min(100, Math.round(100 * c.progress.successes / target)) : 0
  const budgetPct = c.budget.max_calls ? Math.min(100, Math.round(100 * c.progress.calls_placed / c.budget.max_calls)) : 0

  async function stop() {
    if (!confirm('Stop this campaign? Remaining pending audience will be skipped.')) return
    await api.stopCampaign(conn, c.id)
  }

  return (
    <div>
      <div className="page-header">
        <div className="spread">
          <div>
            <h1>🎯 {c.name}</h1>
            <p className="mono">{c.id} · {c.event_type}</p>
          </div>
          <div className="row">
            <span className={`pill ${STATUS_TONE[c.status] || 'muted'}`}>
              {c.status === 'running' && <span className="dot" />} {c.status}
            </span>
            {c.status === 'running' && <button className="btn danger sm" onClick={stop}>Stop</button>}
            <Link to="/campaigns" className="btn sm">← all campaigns</Link>
          </div>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 18 }}>
        <div className="spread" style={{ marginBottom: 8 }}>
          <strong style={{ fontSize: 15 }}>
            {c.goal.type === 'count' ? `${c.progress.successes} / ${target} successes` : `reach all — ${c.progress.pending} pending`}
          </strong>
          <span className="faint">{c.goal.success_outcomes.join(', ')}</span>
        </div>
        <div className="bar ok"><div className="fill" style={{ width: `${pct}%` }} /></div>
        <div className="row" style={{ marginTop: 14, justifyContent: 'space-between' }}>
          <span className="pill muted">wave {c.wave_number} · size {c.waves.size} · delay {c.waves.delay}</span>
          {c.next_wave_at && c.status === 'running' && (
            <span className="pill warn">next wave {new Date(c.next_wave_at).toLocaleTimeString()}</span>
          )}
        </div>
      </div>

      <div className="stat-grid">
        <div className="card stat ok"><div className="label">Successes</div><div className="value">{c.progress.successes}</div></div>
        <div className="card stat"><div className="label">Calls placed</div><div className="value">{c.progress.calls_placed}</div></div>
        <div className="card stat"><div className="label">Pending</div><div className="value">{c.progress.pending}</div></div>
        <div className="card stat warn"><div className="label">No answer</div><div className="value">{c.progress.no_answer}</div></div>
        <div className={`card stat ${c.progress.failures ? 'err' : ''}`}><div className="label">Failed</div><div className="value">{c.progress.failures}</div></div>
      </div>

      <div className="card" style={{ marginBottom: 18 }}>
        <div className="spread" style={{ marginBottom: 8 }}>
          <strong>Budget</strong>
          <span className="mono muted">{c.progress.calls_placed} / {c.budget.max_calls} calls</span>
        </div>
        <div className={`bar ${budgetPct > 85 ? '' : 'ok'}`}><div className="fill" style={{ width: `${budgetPct}%` }} /></div>
      </div>

      <div className="grid2">
        <div>
          <div className="section-title">Audience — {c.audience.length} people</div>
          <AudienceGrid audience={c.audience} />
          <div className="row" style={{ marginTop: 10, fontSize: 11 }}>
            <span className="aud-cell succeeded" style={{ width: 14, height: 14 }} /> succeeded
            <span className="aud-cell no_answer" style={{ width: 14, height: 14 }} /> no answer
            <span className="aud-cell in_wave" style={{ width: 14, height: 14 }} /> in wave
            <span className="aud-cell failed" style={{ width: 14, height: 14 }} /> failed
            <span className="aud-cell pending" style={{ width: 14, height: 14 }} /> pending
            <span className="aud-cell skipped" style={{ width: 14, height: 14 }} /> skipped
          </div>
        </div>
        <div>
          <div className="section-title">Wave log</div>
          <WaveTimeline log={c.log} />
        </div>
      </div>
    </div>
  )
}
