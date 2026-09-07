import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './hooks/useAuth'
import AppShell from './components/layout/AppShell'
import Connect from './pages/Connect'
import WarRoom from './pages/WarRoom'
import Sessions from './pages/Sessions'
import SessionDetail from './pages/SessionDetail'
import Campaigns from './pages/Campaigns'
import CampaignDetail from './pages/CampaignDetail'
import Fire from './pages/Fire'
import Settings from './pages/Settings'

function Gate() {
  const { connected } = useAuth()
  if (!connected) return <Connect />
  return (
    <HashRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<WarRoom />} />
          <Route path="/sessions" element={<Sessions />} />
          <Route path="/sessions/:id" element={<SessionDetail />} />
          <Route path="/campaigns" element={<Campaigns />} />
          <Route path="/campaigns/:id" element={<CampaignDetail />} />
          <Route path="/fire" element={<Fire />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </HashRouter>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <Gate />
    </AuthProvider>
  )
}
