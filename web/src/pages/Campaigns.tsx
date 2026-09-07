import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { usePolling } from '../hooks/usePolling'
import { api } from '../lib/api'
import CampaignWizard from '../components/domain/CampaignWizard'

const STATUS_TONE: Record<string, string> = {
  running: 'warn', goal_met: 'ok', exhausted: 'muted', budget_exhausted: 'err', stopped: 'muted',
}

export default function Campaigns() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const { data: campaigns, setData, stale: campaignsStale } = usePolling(() => api.campaigns(conn), 2500)

  return (
    <div>
      <div className="page-header">
        <h1>Campaigns</h1>
        <p>Declare a goal, not a dial list. Waves stop the moment the goal is met.</p>
      </div>

      <div style={{ marginBottom: 18 }}>
        <CampaignWizard onCreated={c => setData([c, ...(campaigns || [])])} />
      </div>

      {(campaigns === null || campaignsStale) && (
        <div style={{ padding: 20 }} aria-busy="true">
          {[0, 1, 2].map(i => (
            <div key={i} className="skeleton" style={{ height: 92, marginBottom: 14 }} />
          ))}
        </div>
      )}
      {campaigns !== null && !campaignsStale && campaigns.length === 0 && <div className="empty">no campaigns yet — launch your first one above</div>}

      {campaigns !== null && !campaignsStale && campaigns.map(c => {
        const target = c.goal.type === 'count' ? c.goal.target : c.audience.length
        const pct = target ? Math.min(100, Math.round(100 * c.progress.successes / target)) : 0
        return (
          <Link to={`/campaigns/${c.id}`} key={c.id} style={{ display: 'block', marginBottom: 14 }}>
            <div className="card">
              <div className="spread">
                <div className="row">
                  <strong style={{ fontSize: 15 }}>{c.name}</strong>
                  <span className={`pill ${STATUS_TONE[c.status] || 'muted'}`}>
                    {c.status === 'running' && <span className="dot" />} {c.status}
                  </span>
                  <span className="pill muted">{c.event_type}</span>
                </div>
                <span className="faint mono" style={{ fontSize: 11.5 }}>{c.id}</span>
              </div>
              <div className="bar ok" style={{ marginTop: 14 }}>
                <div className="fill" style={{ width: `${pct}%` }} />
              </div>
              <div className="row" style={{ marginTop: 10, fontSize: 12.5, justifyContent: 'space-between' }}>
                <span className="muted">
                  <strong style={{ color: 'var(--text-primary)' }}>{c.progress.successes}</strong>/{target} successes
                   · <strong style={{ color: 'var(--text-primary)' }}>{c.progress.calls_placed}</strong>/{c.budget.max_calls} calls
                   · wave {c.wave_number}
                </span>
                <span className="faint">
                  {c.progress.pending} pending · {c.progress.no_answer} no-answer · {c.progress.failures} failed
                </span>
              </div>
            </div>
          </Link>
        )
      })}
    </div>
  )
}
