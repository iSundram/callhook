import { useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { api } from '../lib/api'
import { EVENT_TYPES, eventDef } from '../lib/events'

export default function Fire() {
  const { baseUrl, token } = useAuth()
  const conn = { baseUrl, token }
  const [type, setType] = useState('invoice.due')
  const [customerId, setCustomerId] = useState('cus_1002')
  const [phone, setPhone] = useState('')
  const [notBefore, setNotBefore] = useState('')
  const [result, setResult] = useState<any>(null)
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function fire() {
    setBusy(true)
    setErr(null)
    setResult(null)
    try {
      const body: any = {
        id: `evt_web_${Date.now()}`,
        type,
        customer_id: customerId,
        payload: eventDef(type).demo,
      }
      if (phone) body.phone = phone
      if (notBefore) body.not_before = new Date(notBefore).toISOString()
      setResult(await api.fire(conn, body))
    } catch (e: any) {
      setErr(e.message)
    } finally {
      setBusy(false)
    }
  }

  const info = eventDef(type)

  return (
    <div>
      <div className="page-header">
        <h1>Fire an event</h1>
        <p>Push one event through the pipeline: prefetch → call → outcome → actions.</p>
      </div>

      <div className="grid2">
        <div className="card">
          <div className="field">
            <label>Event type</label>
            <select className="select" value={type} onChange={e => setType(e.target.value)}>
              {EVENT_TYPES.map(e => <option key={e.type} value={e.type}>{e.label}</option>)}
            </select>
          </div>
          <div className="field">
            <label>Customer ID</label>
            <input className="input mono" value={customerId} onChange={e => setCustomerId(e.target.value)} placeholder="cus_1002" />
            <div className="hint">Demo IDs: cus_1001 (IN), cus_1002 (US), cus_1003 (SG), cus_2001–2030 (campaign audience).</div>
          </div>
          <div className="field">
            <label>Phone override (E.164)</label>
            <input className="input mono" value={phone} onChange={e => setPhone(e.target.value)} placeholder="defaults to customer's number" />
          </div>
          <div className="field">
            <label>Not before (optional)</label>
            <input className="input" type="datetime-local" value={notBefore} onChange={e => setNotBefore(e.target.value)} />
          </div>
          <button className="btn primary" disabled={busy} onClick={fire}>
            {busy ? 'Firing…' : 'Fire event'}
          </button>
          {err && <div className="pill err" style={{ marginTop: 12 }}>{err}</div>}
          {result && (
            <div style={{ marginTop: 16 }}>
              <span className={`pill ${result.status === 'error' ? 'err' : 'ok'}`}>{result.status}</span>
              {result.call_id && <pre className="code" style={{ marginTop: 10 }}>{JSON.stringify(result, null, 2)}</pre>}
            </div>
          )}
        </div>

        <div>
          <div className="card">
            <strong>{info.label}</strong>
            <p className="muted" style={{ fontSize: 13, margin: '8px 0' }}>{info.desc}</p>
            <div className="section-title" style={{ marginTop: 14, marginBottom: 6 }}>Structured outcomes</div>
            <p className="mono" style={{ fontSize: 12, color: 'var(--brand-silver)' }}>{info.outcomes.join(' · ')}</p>
          </div>
          <div className="card" style={{ marginTop: 14 }}>
            <strong>Tip</strong>
            <p className="muted" style={{ fontSize: 13, marginTop: 8 }}>
              To call many people at once with a goal and early-stop, use a{' '}
              <a href="#/campaigns">campaign</a> instead — waves, budget caps, and it stops the moment the goal is met.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
