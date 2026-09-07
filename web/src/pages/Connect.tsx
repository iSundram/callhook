import { useState } from 'react'
import { useAuth } from '../hooks/useAuth'

export default function Connect() {
  const { connect } = useAuth()
  const [baseUrl, setBaseUrl] = useState(window.location.origin.startsWith('http') && !window.location.origin.includes('5173') ? window.location.origin : 'http://localhost:8080')
  const [token, setToken] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    setBusy(true)
    setErr(null)
    try {
      await connect(baseUrl, token)
    } catch (e: any) {
      setErr(e.message || 'could not reach server')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="connect-wrap">
      <div className="connect-card">
        <div className="logo"><img src="/icon-tight.svg" alt="callhook" /></div>
        <div className="card">
          <h2 style={{ fontSize: 17, marginBottom: 4 }}>Connect to your server</h2>
          <p className="muted" style={{ fontSize: 12.5, marginBottom: 18 }}>
            callhook war room — events, calls and campaigns, live.
          </p>
          <div className="field">
            <label>Server URL</label>
            <input className="input mono" value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="http://localhost:8080" />
          </div>
          <div className="field">
            <label>API token</label>
            <input className="input mono" type="password" value={token} onChange={e => setToken(e.target.value)} placeholder="CALLHOOK_INTAKE_TOKEN (if set)" />
            <div className="hint">Leave empty if your server has no intake token.</div>
          </div>
          {err && <div className="pill err" style={{ marginBottom: 12 }}>{err}</div>}
          <button className="btn primary" style={{ width: '100%' }} disabled={busy} onClick={submit}>
            {busy ? 'Connecting…' : 'Connect'}
          </button>
        </div>
        <p className="faint" style={{ textAlign: 'center', fontSize: 11.5, marginTop: 14 }}>
          Fire a webhook. Your customer's phone rings.
        </p>
      </div>
    </div>
  )
}
