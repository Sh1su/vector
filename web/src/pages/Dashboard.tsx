import { useState } from 'react'
import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon, type IconName } from '../components/Icon'
import { Button, Card, CardTitle, Chip, EmptyState } from '../components/ui'
import { carrierLabel, fmtDate, fmtDisplay, fmtNumber } from '../lib/format'
import { dueText, levelInfo } from '../lib/due'
import { sourceInfo, useCurrent, useReadings } from '../lib/odometer'
import { useApp, vehicleSubtitle } from '../lib/state'
import { NewVehicleDialog } from './Vehicles'

function monthStart() {
  const n = new Date()
  return new Date(n.getFullYear(), n.getMonth(), 1)
}

interface Kpi { icon: IconName; label: string; value: string; note: string; to: string }

export function DashboardPage() {
  const { vehicle } = useApp()
  const [open, setOpen] = useState(false)
  const vid = vehicle?.id
  const current = useCurrent(vid).data
  const readings = useReadings(vid).data?.items ?? []
  const month = useQuery({
    enabled: !!vid, queryKey: ['odometer', vid, 'month-distance'],
    queryFn: () => api.get<Schemas['OdometerDistance']>(`/vehicles/${vid}/odometer/distance?from=${encodeURIComponent(monthStart().toISOString())}&to=${encodeURIComponent(new Date().toISOString())}`),
  }).data
  const carrier = vehicle?.energy_carriers?.[0]
  const fuel = useQuery({
    enabled: !!vid && !!carrier, queryKey: ['fuel', vid, 'summary', carrier],
    queryFn: () => api.get<Schemas['ConsumptionSummary']>(`/vehicles/${vid}/fuel/consumption?energy_carrier=${carrier}`),
  }).data
  const oilSeries = useQuery({ enabled: !!vid, queryKey: ['oil', vid, 'series'], queryFn: () => api.get<Schemas['OilSeriesPage']>(`/vehicles/${vid}/oil/series`) }).data
  const due = useQuery({ enabled: !!vid, queryKey: ['maintenance', vid, 'status'], queryFn: () => api.get<Schemas['DueStatusPage']>(`/vehicles/${vid}/maintenance/status`) }).data

  if (!vehicle) {
    return (
      <>
        <Header title="Übersicht" />
        <main className="p-8">
          <EmptyState icon="car" title="Willkommen bei Vectra" text="Lege dein erstes Fahrzeug an. Danach kannst du Kilometerstände, Tankvorgänge, Öl und Wartungen erfassen."
            action={<Button icon="plus" onClick={() => setOpen(true)}>Fahrzeug hinzufügen</Button>} />
        </main>
        <NewVehicleDialog open={open} onOpenChange={setOpen} />
      </>
    )
  }

  const oil = oilSeries?.items[0]
  const known = current && current.kind !== 'unknown' && current.meter_value
  const monthName = new Intl.DateTimeFormat('de-DE', { month: 'long' }).format(new Date())
  const kpis: Kpi[] = [
    { icon: 'gauge', label: 'Kilometerstand', value: known ? `${fmtNumber(current.meter_value!.canonical / 1000)} km` : '–', note: known ? `Stand vom ${fmtDate(current.at, true)}` : 'Noch kein Stand erfasst', to: '/kilometer' },
    { icon: 'route', label: `Gefahren im ${monthName}`, value: month?.status === 'known' && month.distance ? `${fmtNumber(month.distance.canonical / 1000)} km` : '–', note: month?.status !== 'known' ? 'Zu wenige Messpunkte' : month.flags?.includes('confirmed_anomaly_in_range') ? 'enthält eine bestätigte Abweichung' : 'aus deinen Kilometerständen', to: '/kilometer' },
    { icon: 'fuel', label: 'Ø Verbrauch', value: fmtDisplay((carrier === 'electricity' ? fuel?.grid : fuel?.consumption)?.average), note: carrier ? `${carrierLabel[carrier]}, über alle Vollbetankungen` : 'kein Energieträger hinterlegt', to: '/kraftstoff' },
    { icon: 'oil', label: 'Ölverbrauch', value: oil?.status === 'ok' ? fmtDisplay(oil.consumption_per_1000, 0) : '–', note: oil?.status === 'ok' ? 'seit dem letzten Ölwechsel' : 'zu wenige Ölstandsmessungen', to: '/oel' },
  ]
  const urgent = (due?.items ?? []).filter((d) => d.level === 'overdue' || d.level === 'due' || d.level === 'upcoming')

  return (
    <>
      <Header title="Übersicht" sub={`${vehicle.display_name} · ${vehicleSubtitle(vehicle)}`}
        actions={<Link to="/kilometer" className="inline-flex h-10 items-center gap-2 rounded-[12px] bg-teal px-4 text-sm font-semibold whitespace-nowrap text-ink"><Icon name="plus" size={18} />Kilometerstand erfassen</Link>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        <div className="flex items-center gap-3.5 rounded-[16px] bg-hero px-5 py-4">
          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-ink ${urgent.some((d) => d.level !== 'upcoming') ? 'bg-amber' : 'bg-ready'}`}><Icon name={urgent.some((d) => d.level !== 'upcoming') ? 'alert' : 'check'} size={22} /></div>
          <div className="flex flex-grow flex-col gap-0.5">
            <div className="font-display text-[19px] font-semibold text-paper">{urgent.some((d) => d.level !== 'upcoming') ? `${urgent.filter((d) => d.level !== 'upcoming').length} Wartung(en) fällig oder überfällig.` : 'Dein Fahrzeug ist bereit für die nächste Fahrt.'}</div>
            <div className="text-[13px] text-slate-300">{known ? `Letzter Stand ${fmtNumber(current.meter_value!.canonical / 1000)} km am ${fmtDate(current.at, true)}` : 'Erfasse den ersten Kilometerstand.'}</div>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {kpis.map((k) => (
            <Link key={k.label} to={k.to} className="flex flex-col gap-1.5 rounded-[16px] border border-line bg-card p-4 text-text">
              <div className="flex items-center gap-2 text-[13px] font-medium text-muted"><span className="text-link"><Icon name={k.icon} /></span>{k.label}</div>
              <div className="tabular font-display text-[26px] font-semibold">{k.value}</div>
              <div className="text-xs text-muted">{k.note}</div>
            </Link>
          ))}
        </div>

        <div className="grid min-h-0 grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
          <Card className="flex flex-col gap-3.5 p-5">
            <CardTitle aside={<Link to="/kilometer" className="flex items-center gap-0.5 text-sm font-semibold text-link">Alle Stände<Icon name="right" size={16} /></Link>}>Letzte Kilometerstände</CardTitle>
            {readings.length === 0 && <div className="text-sm text-muted">Noch keine Einträge.</div>}
            <div className="flex flex-col">
              {readings.slice(0, 5).map((r, i, arr) => {
                const src = sourceInfo[r.source] ?? sourceInfo.manual
                return (
                  <div key={r.id} className="flex gap-3.5">
                    <div className="flex flex-col items-center">
                      <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name={src.icon} /></div>
                      <div className={`w-0.5 flex-grow ${i === arr.length - 1 ? 'bg-transparent' : 'bg-line'}`} />
                    </div>
                    <div className="flex flex-grow justify-between gap-3 pt-1.5 pb-4">
                      <div className="flex flex-col gap-0.5"><div className="tabular text-[15px] font-semibold">{fmtNumber(r.meter_value.canonical / 1000)} km</div><div className="text-[13px] text-muted">{src.label} · {fmtDate(r.occurred_at, r.time_precision === 'date_only')}</div></div>
                      {r.status === 'confirmed_anomaly' && <Chip tone="warn" icon="alert">bestätigte Abweichung</Chip>}
                    </div>
                  </div>
                )
              })}
            </div>
          </Card>
          <div className="flex flex-col gap-5">
            <Card className="flex flex-col gap-3 p-5">
              <CardTitle aside={<Link to="/wartung" className="flex items-center gap-0.5 text-sm font-semibold text-link">Alle<Icon name="right" size={16} /></Link>}>Nächste Wartungen</CardTitle>
              {(due?.items ?? []).length === 0 && <div className="text-sm text-muted">Noch kein Wartungsplan. <Link to="/wartung" className="font-semibold text-link">Vorlage übernehmen</Link></div>}
              {(due?.items ?? []).slice(0, 4).map((d) => (
                <div key={d.item_id} className="flex items-center justify-between gap-3">
                  <div className="flex min-w-0 flex-col gap-0.5"><div className="truncate text-sm font-semibold">{d.title}</div><div className="truncate text-xs text-muted">{dueText(d)}</div></div>
                  <Chip tone={levelInfo[d.level].tone}>{levelInfo[d.level].label}</Chip>
                </div>
              ))}
            </Card>
            <Card className="flex flex-col gap-3 p-5">
              <CardTitle>Schnell erfassen</CardTitle>
              <div className="grid grid-cols-2 gap-2.5">
                <Link to="/kilometer" className="flex h-16 items-center gap-2.5 rounded-[14px] border border-line bg-soft px-3.5 text-sm font-semibold text-text"><span className="text-link"><Icon name="gauge" size={22} /></span>km-Stand</Link>
                {([['fuel', 'Tanken', '/kraftstoff'], ['oil', 'Öl', '/oel'], ['sparkles', 'Assistent', '/assistent']] as [IconName, string, string][]).map(([icon, label, to]) => (
                  <Link key={label} to={to} className="flex h-16 items-center gap-2.5 rounded-[14px] border border-line bg-soft px-3.5 text-sm font-semibold text-text"><span className="text-link"><Icon name={icon} size={22} /></span>{label}</Link>
                ))}
              </div>
            </Card>
            <Link to="/fahrzeuge" className="flex items-center gap-3 rounded-[16px] border-[1.5px] border-dashed border-line px-4 py-3.5 text-text">
              <span className="text-link"><Icon name="garage" size={22} /></span>
              <div className="flex flex-grow flex-col gap-0.5"><div className="text-sm font-semibold">Fahrzeuge verwalten</div><div className="text-xs text-muted">Anlegen, wechseln, teilen</div></div>
              <span className="text-muted"><Icon name="right" size={18} /></span>
            </Link>
          </div>
        </div>
      </main>
    </>
  )
}
