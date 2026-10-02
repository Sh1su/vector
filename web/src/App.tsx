import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api, ProblemError } from './api/client'
import { Sidebar } from './components/Sidebar'
import { AppStateProvider, type Account } from './lib/state'
import { LoginPage, SetupPage } from './pages/Login'
import { DashboardPage } from './pages/Dashboard'
import { VehiclesPage } from './pages/Vehicles'
import { OdometerPage } from './pages/Odometer'
import { TripsPage } from './pages/Trips'
import { MaintenancePage } from './pages/Maintenance'
import { ServicePage } from './pages/Service'
import { CostsPage } from './pages/Costs'
import { DocumentsPage } from './pages/Documents'
import { SettingsPage } from './pages/Settings'
import { AssistantPage } from './pages/Assistant'

// Kraftstoff und Öl werden bei Bedarf nachgeladen (Bundle-Budget, ADR-022).
const FuelPage = lazy(() => import('./pages/Fuel').then((m) => ({ default: m.FuelPage })))
const OilPage = lazy(() => import('./pages/Oil').then((m) => ({ default: m.OilPage })))

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
            <Route path="kraftstoff" element={<Suspense fallback={<div className="p-8 text-muted">Lade …</div>}><FuelPage /></Suspense>} />
            <Route path="oel" element={<Suspense fallback={<div className="p-8 text-muted">Lade …</div>}><OilPage /></Suspense>} />
            <Route path="dokumente" element={<DocumentsPage />} />
            <Route path="assistent" element={<AssistantPage />} />
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
