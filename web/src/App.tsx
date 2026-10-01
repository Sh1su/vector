import { Navigate, Route, Routes } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api, ProblemError } from './api/client'
import { Sidebar } from './components/Sidebar'
import { AppStateProvider, type Account } from './lib/state'
import { LoginPage, SetupPage } from './pages/Login'
import { DashboardPage } from './pages/Dashboard'
import { VehiclesPage } from './pages/Vehicles'
import { OdometerPage } from './pages/Odometer'
import { ComingSoon } from './pages/ComingSoon'
import { TripsPage } from './pages/Trips'
import { MaintenancePage } from './pages/Maintenance'
import { ServicePage } from './pages/Service'
import { CostsPage } from './pages/Costs'
import { DocumentsPage } from './pages/Documents'
import { SettingsPage } from './pages/Settings'

function Shell() {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.get<Account>('/me'), retry: false })
  if (me.isPending) return <div className="flex h-full items-center justify-center text-muted">Lade …</div>
  if (me.error) {
    if (me.error instanceof ProblemError && me.error.status === 401) return <Navigate to="/login" replace />
    return <div role="alert" className="p-8 text-bad">Server nicht erreichbar.</div>
  }
  return (
    <AppStateProvider>
      <div className="flex h-full overflow-hidden">
        <Sidebar me={me.data} />
        <div className="flex min-w-0 flex-grow flex-col">
          <Routes>
            <Route index element={<DashboardPage />} />
            <Route path="fahrzeuge" element={<VehiclesPage />} />
            <Route path="kilometer" element={<OdometerPage />} />
            <Route path="fahrten" element={<TripsPage />} />
            <Route path="wartung" element={<MaintenancePage />} />
            <Route path="service" element={<ServicePage />} />
            <Route path="kosten" element={<CostsPage />} />
            <Route path="kraftstoff" element={<ComingSoon title="Kraftstoff" icon="fuel" iteration={2} />} />
            <Route path="oel" element={<ComingSoon title="Öl" icon="oil" iteration={2} />} />
            <Route path="dokumente" element={<DocumentsPage />} />
            <Route path="assistent" element={<ComingSoon title="Assistent" icon="sparkles" iteration={7} />} />
            <Route path="einstellungen" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </div>
    </AppStateProvider>
  )
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/einrichtung" element={<SetupPage />} />
      <Route path="/*" element={<Shell />} />
    </Routes>
  )
}
