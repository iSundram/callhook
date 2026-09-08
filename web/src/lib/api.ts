// Typed client for the callhook backend API.
import { track } from './loading'

export type Conn = { baseUrl: string; token: string }

async function request<T>(conn: Conn, path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (conn.token) headers['Authorization'] = `Bearer ${conn.token}`
  const res = await fetch(`${conn.baseUrl.replace(/\/$/, '')}${path}`, {
    ...init,
    headers: { ...headers, ...(init?.headers as any) },
  })
  const body = await res.text()
  let data: any = null
  try { data = body ? JSON.parse(body) : null } catch { /* non-JSON */ }
  if (!res.ok) {
    throw new Error(data?.error || `HTTP ${res.status}`)
  }
  return data as T
}

// Mutations are user-initiated: the global progress bar + dim reflect them.
// Reads (polling) stay untracked — their loading shows locally as shimmer.
async function mutation<T>(conn: Conn, path: string, init?: RequestInit): Promise<T> {
  return track(() => request<T>(conn, path, init))
}

export const api = {
  health: (c: Conn) => request<any>(c, '/api/health'),

  sessions: (c: Conn) => request<any[]>(c, '/api/sessions'),
  metrics: (c: Conn) => request<any>(c, '/api/metrics'),

  campaigns: (c: Conn) => request<any[]>(c, '/api/campaigns'),
  campaign: (c: Conn, id: string) => request<any>(c, `/api/campaigns/${id}`),
  createCampaign: (c: Conn, body: any) =>
    mutation<any>(c, '/api/campaigns', { method: 'POST', body: JSON.stringify(body) }),
  stopCampaign: (c: Conn, id: string) =>
    mutation<any>(c, `/api/campaigns/${id}/stop`, { method: 'POST' }),

  fire: (c: Conn, body: any) =>
    mutation<any>(c, '/api/events', { method: 'POST', body: JSON.stringify(body) }),
  fireBatch: (c: Conn, events: any[]) =>
    mutation<any>(c, '/api/events/batch', { method: 'POST', body: JSON.stringify({ events }) }),

  integrations: (c: Conn) => request<any>(c, '/api/integrations'),
}
