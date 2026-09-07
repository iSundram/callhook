// Typed client for the callhook backend API.

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

export const api = {
  health: (c: Conn) => request<any>(c, '/api/health'),

  sessions: (c: Conn) => request<any[]>(c, '/api/sessions'),
  metrics: (c: Conn) => request<any>(c, '/api/metrics'),

  campaigns: (c: Conn) => request<any[]>(c, '/api/campaigns'),
  campaign: (c: Conn, id: string) => request<any>(c, `/api/campaigns/${id}`),
  createCampaign: (c: Conn, body: any) =>
    request<any>(c, '/api/campaigns', { method: 'POST', body: JSON.stringify(body) }),
  stopCampaign: (c: Conn, id: string) =>
    request<any>(c, `/api/campaigns/${id}/stop`, { method: 'POST' }),

  fire: (c: Conn, body: any) =>
    request<any>(c, '/api/events', { method: 'POST', body: JSON.stringify(body) }),
  fireBatch: (c: Conn, events: any[]) =>
    request<any>(c, '/api/events/batch', { method: 'POST', body: JSON.stringify({ events }) }),
}
