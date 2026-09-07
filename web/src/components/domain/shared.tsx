import type { Turn } from '../../lib/types'
import { clock } from '../../lib/format'
import { OUTCOME_TONE } from '../../lib/format'

export function OutcomeBadge({ outcome }: { outcome: string | undefined }) {
  if (!outcome) return <span className="pill muted">—</span>
  const tone = OUTCOME_TONE[outcome] || 'muted'
  return <span className={`pill ${tone}`}>{outcome}</span>
}

export function StatusPill({ status }: { status: string }) {
  const s = status || 'intake'
  const tone = s === 'completed' ? 'ok' : s === 'failed' || s === 'canceled' ? 'err' : s === 'in_progress' ? 'warn' : 'muted'
  return <span className={`pill ${tone}`}>{s}</span>
}

export function TranscriptView({ turns }: { turns: Turn[] | null | undefined }) {
  if (!turns || turns.length === 0) return <div className="empty">no transcript available</div>
  return (
    <div className="transcript">
      {turns.map((t, i) => (
        <div key={i} className={`turn ${t.speaker}`}>
          <span className="who">{t.speaker === 'bot' ? 'AGENT' : t.speaker === 'user' ? 'CUSTOMER' : '??'}:</span>
          <span>{t.text}</span>
          {t.offset_seconds !== null && <span className="when">{t.offset_seconds}s</span>}
        </div>
      ))}
    </div>
  )
}

export function ActionTimeline({ actions }: { actions: { at: string; kind: string; detail: string }[] }) {
  if (!actions || actions.length === 0) return <div className="empty">no actions yet</div>
  return (
    <div className="timeline">
      {[...actions].reverse().map((a, i) => (
        <div key={i} className="tl-item">
          <div className="t">{clock(a.at)}</div>
          <div className="k">{a.kind}</div>
          <div className="d">{a.detail}</div>
        </div>
      ))}
    </div>
  )
}

export function AudienceGrid({ audience }: { audience: any[] }) {
  return (
    <div className="aud-grid">
      {audience.map((e, i) => (
        <div key={i} className={`aud-cell ${e.state}`} title={`${e.customer_id} — ${e.state}${e.rounds ? ` (round ${e.rounds})` : ''}`}>
          {e.customer_id.replace(/^cus_/, '')}
        </div>
      ))}
    </div>
  )
}

export function WaveTimeline({ log }: { log: { at: string; event: string; note?: string }[] }) {
  if (!log || log.length === 0) return <div className="empty">no wave activity yet</div>
  return (
    <div className="timeline">
      {[...log].reverse().map((l, i) => (
        <div key={i} className="tl-item">
          <div className="t">{clock(l.at)}</div>
          <div className="k">{l.event}</div>
          {l.note && <div className="d">{l.note}</div>}
        </div>
      ))}
    </div>
  )
}
