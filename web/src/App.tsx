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
import { ComingSoon } from './pages/ComingSoon'

// Fachseiten werden bei Bedarf nachgeladen (Bundle-Budget, ADR-022).
const FuelPage = lazy(() => import('./pages/Fuel').then((m) => ({ default: m.FuelPage })))
const OilPage = lazy(() => import('./pages/Oil').then((m) => ({ default: m.OilPage })))
const MaintenancePage = lazy(() => import('./pages/Maintenance').then((m) => ({ default: m.MaintenancePage })))
const AssistantPage = lazy(() => import('./pages/Assistant').then((m) => ({ default: m.AssistantPage })))
const SettingsPage = lazy(() => import('./pages/Settings').then((m) => ({ default: m.SettingsPage })))

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
          <Suspense fallback={<div className="p-8 text-muted">Lade …</div>}>
          <Routes>
            <Route index element={<DashboardPage />} />
            <Route path="fahrzeuge" element={<VehiclesPage />} />
            <Route path="kilometer" element={<OdometerPage />} />
            <Route path="fahrten" element={<ComingSoon title="Fahrten" icon="route" iteration={3} />} />
            <Route path="wartung" element={<MaintenancePage />} />
            <Route path="service" element={<ComingSoon title="Servicehistorie" icon="receipt" iteration={2} />} />
            <Route path="kosten" element={<ComingSoon title="Kosten" icon="euro" iteration={2} />} />
            <Route path="kraftstoff" element={<FuelPage />} />
            <Route path="oel" element={<OilPage />} />
            <Route path="dokumente" element={<ComingSoon title="Dokumente" icon="doc" iteration={3} />} />
            <Route path="assistent" element={<AssistantPage />} />
            <Route path="einstellungen" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
          </Suspense>
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
