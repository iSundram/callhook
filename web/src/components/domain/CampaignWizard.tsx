import { useState } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../hooks/useAuth'
import { ErrorDocsHint } from './DocsHint'
import { EVENT_TYPES, eventDef } from '../../lib/events'

// Per-type presets: name + goal sized to the type's natural success outcome.
const presetFor = (type: string) => {
  const def = eventDef(type)
  const names: Record<string, string> = {
    'invoice.due': 'September collections',
    'account.warning': 'Security sweep',
    'promo.offer': 'Loyalty push',
    'delivery.window': 'Delivery confirmations',
    'appointment.reminder': 'Appointment confirmations',
    'payment.failed': 'Failed payment recovery',
    'subscription.expiring': 'Renewal push',
    'feedback.request': 'Feedback round',
  }
  const targets: Record<string, number> = {
    'invoice.due': 5, 'account.warning': 10, 'promo.offer': 8,
    'delivery.window': 6, 'appointment.reminder': 6, 'payment.failed': 5,
    'subscription.expiring': 6, 'feedback.request': 10,
  }
  return {
    name: names[type] || def.label,
    goal_type: 'count',
    target: targets[type] || 5,
    success: def.successOutcome,
    size: 3,
    delay: '30s',
    max_waves: 6,
    max_calls: 15,
  }
}

export default function CampaignWizard({ onCreated }: { onCreated: (c: any) => void }) {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: 'September collections',
    event_type: 'invoice.due',
    goal_type: 'count',
    target: 5,
    success: 'payment_promised',
    audience_source: 'all_overdue',
    size: 3,
    delay: '30s',
    max_waves: 6,
    max_calls: 15,
  })

  if (!open) return <button className="btn primary" onClick={() => setOpen(true)}>+ New Campaign</button>

  async function create() {
    setBusy(true)
    setErr(null)
    try {
      const body: any = {
        name: form.name,
        event_type: form.event_type,
        goal: {
          type: form.goal_type,
          success_outcomes: [form.success],
          ...(form.goal_type === 'count' ? { target: Number(form.target) } : {}),
        },
        audience_source: form.audience_source,
        waves: { size: Number(form.size), delay: form.delay, max_waves: Number(form.max_waves) },
        budget: { max_calls: Number(form.max_calls) },
      }
      const c = await api.createCampaign(conn, body)
      setOpen(false)
      onCreated(c)
    } catch (e: any) {
      setErr(e.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card" style={{ marginBottom: 18 }}>
      <div className="spread" style={{ marginBottom: 14 }}>
        <strong>New campaign</strong>
        <div className="row">
          <button className="btn sm" onClick={() => setForm({ ...form, ...presetFor(form.event_type) })}>Load preset</button>
          <button className="btn sm" onClick={() => setOpen(false)}>Cancel</button>
        </div>
      </div>

      <div className="grid2">
        <div>
          <div className="field">
            <label>Name</label>
            <input className="input" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
          </div>
          <div className="field">
            <label>Event type</label>
            <select className="select" value={form.event_type} onChange={e => setForm({ ...form, event_type: e.target.value, ...presetFor(e.target.value) })}>
              {EVENT_TYPES.map(e => <option key={e.type} value={e.type}>{e.label}</option>)}
            </select>
          </div>
          <div className="field">
            <label>Goal type</label>
            <select className="select" value={form.goal_type} onChange={e => setForm({ ...form, goal_type: e.target.value })}>
              <option value="count">count — stop at N successes</option>
              <option value="reach_all">reach_all — contact everyone</option>
            </select>
          </div>
          {form.goal_type === 'count' && (
            <div className="field">
              <label>Target successes</label>
              <input className="input" type="number" min={1} value={form.target} onChange={e => setForm({ ...form, target: Number(e.target.value) })} />
            </div>
          )}
          <div className="field">
            <label>Success outcome</label>
            <select className="select" value={form.success} onChange={e => setForm({ ...form, success: e.target.value })}>
              {eventDef(form.event_type).outcomes.map(o => <option key={o}>{o}</option>)}
            </select>
          </div>
        </div>
        <div>
          <div className="field">
            <label>Audience</label>
            <select className="select" value={form.audience_source} onChange={e => setForm({ ...form, audience_source: e.target.value })}>
              <option value="all_overdue">all_overdue — auto from business store</option>
            </select>
            <div className="hint">Explicit audience lists via the API.</div>
          </div>
          <div className="field">
            <label>Wave size</label>
            <input className="input" type="number" min={1} max={10} value={form.size} onChange={e => setForm({ ...form, size: Number(e.target.value) })} />
            <div className="hint">Capped by CALLHOOK_MAX_CONCURRENT.</div>
          </div>
          <div className="field">
            <label>Delay between waves</label>
            <input className="input" value={form.delay} onChange={e => setForm({ ...form, delay: e.target.value })} />
            <div className="hint">Go duration: "30s", "15m"...</div>
          </div>
          <div className="field">
            <label>Max waves</label>
            <input className="input" type="number" min={1} value={form.max_waves} onChange={e => setForm({ ...form, max_waves: Number(e.target.value) })} />
          </div>
          <div className="field">
            <label>Budget — max calls</label>
            <input className="input" type="number" min={1} value={form.max_calls} onChange={e => setForm({ ...form, max_calls: Number(e.target.value) })} />
            <div className="hint">Hard stop. Never burns more than this.</div>
          </div>
        </div>
      </div>

      {err && (
        <div style={{ marginBottom: 10 }}>
          <div className="pill err">{err}</div>
          <ErrorDocsHint message={err} />
        </div>
      )}
      <button className="btn primary" disabled={busy} onClick={create}>
        {busy ? 'Launching…' : 'Launch campaign'}
      </button>
    </div>
  )
}
