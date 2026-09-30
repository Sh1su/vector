import { useQuery } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'

// Lesefelder sind in der Spezifikation readOnly und damit im Typ optional; der Server liefert sie immer.
export type Reading = Schemas['OdometerReading'] & { meter_value: Schemas['Quantity']; total: Schemas['Quantity']; source: string; status: string }
export type OdoValue = Schemas['OdometerValue']

export const odoKeys = (vid: string) => ['odometer', vid]

export function useCurrent(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: [...odoKeys(vid!), 'current'], queryFn: () => api.get<OdoValue>(`/vehicles/${vid}/odometer/current`) })
}

export function useReadings(vid?: string, includeSuperseded = false) {
  return useQuery({
    enabled: !!vid, queryKey: [...odoKeys(vid!), 'readings', includeSuperseded],
    queryFn: () => api.get<{ items: Reading[]; next_cursor: string | null }>(`/vehicles/${vid}/odometer/readings?limit=100&include_superseded=${includeSuperseded}`),
  })
}

export interface MonthBar { key: string; label: string; km: number | null }

/** Gefahrene Strecke der letzten n Monate über ODO-05 (je Monat eine Abfrage). */
export function useMonthly(vid?: string, n = 6) {
  return useQuery({
    enabled: !!vid, queryKey: [...odoKeys(vid!), 'monthly', n],
    queryFn: async (): Promise<MonthBar[]> => {
      const now = new Date()
      const months = Array.from({ length: n }, (_, i) => new Date(now.getFullYear(), now.getMonth() - (n - 1 - i), 1))
      return Promise.all(months.map(async (m) => {
        const from = m
        const to = new Date(m.getFullYear(), m.getMonth() + 1, 1)
        const end = to > now ? now : to
        const d = await api.get<Schemas['OdometerDistance']>(`/vehicles/${vid}/odometer/distance?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(end.toISOString())}`)
        return {
          key: `${m.getFullYear()}-${m.getMonth() + 1}`,
          label: new Intl.DateTimeFormat('de-DE', { month: 'short' }).format(m).replace('.', ''),
          km: d.status === 'known' && d.distance ? d.distance.canonical / 1000 : null,
        }
      }))
    },
  })
}

export const sourceInfo: Record<string, { label: string; icon: 'gauge' | 'fuel' | 'oil' | 'route' | 'wrench' | 'sync' | 'sparkles' }> = {
  manual: { label: 'Manuell erfasst', icon: 'gauge' },
  fuel: { label: 'Tankvorgang', icon: 'fuel' },
  oil: { label: 'Ölmessung', icon: 'oil' },
  trip_start: { label: 'Fahrtbeginn', icon: 'route' },
  trip_end: { label: 'Fahrtende', icon: 'route' },
  service: { label: 'Service', icon: 'wrench' },
  import: { label: 'Import', icon: 'sync' },
  assistant: { label: 'Assistent', icon: 'sparkles' },
}
