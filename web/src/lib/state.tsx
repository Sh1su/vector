import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'

export type Vehicle = Schemas['Vehicle']
export type Account = Schemas['Account']

type Theme = 'light' | 'dark' | 'system'

interface AppState {
  vehicles: Vehicle[]
  vehicle: Vehicle | null
  setVehicleId: (id: string) => void
  theme: Theme
  setTheme: (t: Theme) => void
}

const Ctx = createContext<AppState | null>(null)

function load(key: string) {
  try { return localStorage.getItem(key) } catch { return null }
}
function save(key: string, v: string) {
  try { localStorage.setItem(key, v) } catch { /* privater Modus */ }
}

export function useVehicles() {
  return useQuery({ queryKey: ['vehicles'], queryFn: () => api.get<Schemas['VehiclePage']>('/vehicles?limit=200') })
}

export function AppStateProvider({ children }: { children: ReactNode }) {
  const { data } = useVehicles()
  const vehicles = data?.items ?? []
  const [vehicleId, setVehicleIdState] = useState<string | null>(() => load('vectra.vehicle'))
  const [theme, setThemeState] = useState<Theme>(() => (load('vectra.theme') as Theme) || 'system')
  const vehicle = useMemo(() => vehicles.find((v) => v.id === vehicleId) ?? vehicles[0] ?? null, [vehicles, vehicleId])

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const apply = () => document.documentElement.classList.toggle('dark', theme === 'dark' || (theme === 'system' && mq.matches))
    apply()
    mq.addEventListener('change', apply)
    return () => mq.removeEventListener('change', apply)
  }, [theme])

  const value: AppState = {
    vehicles, vehicle,
    setVehicleId: (id) => { setVehicleIdState(id); save('vectra.vehicle', id) },
    theme,
    setTheme: (t) => { setThemeState(t); save('vectra.theme', t) },
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useApp() {
  const c = useContext(Ctx)
  if (!c) throw new Error('AppStateProvider fehlt')
  return c
}

export function vehicleSubtitle(v: Vehicle) {
  return v.license_plate || 'Ohne Kennzeichen'
}
