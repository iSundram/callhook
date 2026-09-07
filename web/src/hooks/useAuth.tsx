import { createContext, useContext, useState, type ReactNode } from 'react'
import type { Health } from '../lib/types'
import { api } from '../lib/api'

type AuthCtx = {
  baseUrl: string
  token: string
  health: Health | null
  connected: boolean
  connect: (baseUrl: string, token: string) => Promise<void>
  disconnect: () => void
  refreshHealth: () => Promise<void>
}

const Ctx = createContext<AuthCtx>(null as any)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [baseUrl, setBaseUrl] = useState(() => localStorage.getItem('ch_baseurl') || '')
  const [token, setToken] = useState(() => localStorage.getItem('ch_token') || '')
  const [health, setHealth] = useState<Health | null>(null)
  const [connected, setConnected] = useState(false)

  async function connect(url: string, tok: string) {
    const normalized = url.replace(/\/$/, '')
    const h = await api.health({ baseUrl: normalized, token: tok })
    if (!h.ok) throw new Error('server responded but not healthy')
    localStorage.setItem('ch_baseurl', normalized)
    localStorage.setItem('ch_token', tok)
    setBaseUrl(normalized)
    setToken(tok)
    setHealth(h)
    setConnected(true)
  }

  async function refreshHealth() {
    if (!baseUrl) return
    try {
      const h = await api.health({ baseUrl, token })
      setHealth(h)
    } catch { /* keep last known */ }
  }

  function disconnect() {
    localStorage.removeItem('ch_baseurl')
    localStorage.removeItem('ch_token')
    setBaseUrl('')
    setToken('')
    setConnected(false)
    setHealth(null)
  }

  return (
    <Ctx.Provider value={{ baseUrl, token, health, connected, connect, disconnect, refreshHealth }}>
      {children}
    </Ctx.Provider>
  )
}

export const useAuth = () => useContext(Ctx)
