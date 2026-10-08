import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Anomaly, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Card, CardTitle, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { browserTimeZone, fmtDate, fmtNumber, localInputValue } from '../lib/format'
import { errorText, etagOf, parseNumber } from '../lib/money'
import { useCurrent } from '../lib/odometer'
import { useApp } from '../lib/state'
import { fileUrl } from '../lib/files'

type Trip = Schemas['Trip'] & { version: number; status: string; root_id: string }
type Category = Schemas['TripCategory'] & { id: string; version: number }

const kindTone: Record<string, 'info' | 'ok' | 'warn' | 'neutral'> = { business: 'info', commute: 'warn', private: 'ok', other: 'neutral' }

function monthRange() {
  const n = new Date()
  const pad = (x: number) => String(x).padStart(2, '0')
  const last = new Date(n.getFullYear(), n.getMonth() + 1, 0).getDate()
  return { from: `${n.getFullYear()}-${pad(n.getMonth() + 1)}-01`, to: `${n.getFullYear()}-${pad(n.getMonth() + 1)}-${pad(last)}` }
}

export function TripsPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const [dialog, setDialog] = useState<'start' | 'record' | null>(null)
  const [selected, setSelected] = useState<Trip | null>(null)
  const [history, setHistory] = useState(false)
  const trips = useQuery({ enabled: !!vid, queryKey: ['trips', vid, 'list', history], queryFn: () => api.get<{ items: Trip[] }>(`/vehicles/${vid}/trips?limit=100&include_history=${history}`) })
  const cats = useQuery({ enabled: !!vid, queryKey: ['trips', vid, 'categories'], queryFn: () => api.get<{ items: Category[] }>(`/vehicles/${vid}/trip-categories`) })
  const m = monthRange()
  const report = useQuery({ enabled: !!vid, queryKey: ['trips', vid, 'report', m.from], queryFn: () => api.get<Schemas['TripReport']>(`/vehicles/${vid}/trip-report?from=${m.from}&to=${m.to}`) })
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'

  if (!vehicle) return <><Header title="Fahrten" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>
  const items = trips.data?.items ?? []
  const open = items.find((t) => t.status === 'open')
  const catById = new Map((cats.data?.items ?? []).map((c) => [c.id, c]))
  const monthName = new Intl.DateTimeFormat('de-DE', { month: 'long' }).format(new Date())

  return (
    <>
      <Header title="Fahrten" sub={`${vehicle.display_name} · Strecke wird aus den Kilometerständen berechnet`}
        actions={canEdit && <div className="flex gap-2"><Button variant="outline" icon="plus" onClick={() => setDialog('record')}>Fahrt nachtragen</Button>
          {!open && <Button icon="play" onClick={() => setDialog('start')}>Fahrt starten</Button>}</div>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        {open && <OpenTrip vid={vehicle.id} trip={open} category={catById.get(open.category_id)} canEdit={canEdit} />}
        <div className="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
          <Card className="flex flex-col overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4">
              <h2 className="m-0 font-display text-base font-semibold">Fahrtenliste</h2>
              <label className="flex items-center gap-2 text-[13px] text-muted"><input type="checkbox" className="h-4 w-4 accent-teal" checked={history} onChange={(e) => setHistory(e.target.checked)} />mit Korrekturen und Stornos</label>
            </div>
            {items.length === 0 && <div className="border-t border-line px-5 py-8 text-sm text-muted">Noch keine Fahrten erfasst.</div>}
            {items.map((t) => {
              const c = catById.get(t.category_id)
              const invalid = t.status === 'superseded' || t.status === 'cancelled'
              return (
                <div key={t.id}>
                  {t.gap_before && !history && (
                    <div className="flex items-center gap-2 border-t border-dashed border-line bg-warn-bg/50 px-5 py-1.5 text-xs text-warn"><Icon name="alert" size={14} />{fmtNumber(t.gap_before.value)} {t.gap_before.unit} nicht als Fahrt erfasst</div>
                  )}
                  <button type="button" disabled={!canEdit || invalid || t.status === 'open'} onClick={() => setSelected(t)}
                    className={`grid w-full grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-3 text-left text-text hover:bg-soft disabled:hover:bg-transparent ${invalid ? 'opacity-60' : ''}`}>
                    <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name="route" /></div>
                    <div className="flex min-w-0 flex-col gap-0.5">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className={`truncate text-[15px] font-semibold ${invalid ? 'line-through' : ''}`}>{[t.start_location, t.end_location].filter(Boolean).join(' → ') || t.purpose || 'Fahrt'}</span>
                        {c && <Chip tone={kindTone[c.kind]}>{c.name}</Chip>}
                        {t.status === 'open' && <Chip tone="info" icon="clock">läuft</Chip>}
                        {t.status === 'superseded' && <Chip>korrigiert</Chip>}
                        {t.status === 'cancelled' && <Chip tone="bad">storniert</Chip>}
                      </div>
                      <div className="text-xs text-muted">{fmtDate(t.started_at)}{t.ended_at ? ` – ${new Intl.DateTimeFormat('de-DE', { timeStyle: 'short' }).format(new Date(t.ended_at))}` : ''} · {fmtNumber(t.start_odometer.value)}{t.end_odometer ? ` → ${fmtNumber(t.end_odometer.value)}` : ''} km{t.change_reason ? ` · ${t.change_reason}` : ''}</div>
                    </div>
                    <div className="tabular text-[15px] font-semibold">{t.distance ? `${fmtNumber(t.distance.value, 1)} ${t.distance.unit}` : '–'}</div>
                  </button>
                  {vid && (t.start_photo_id || t.end_photo_id) && (
                    <div className="flex gap-4 px-5 pb-3 pl-[72px] text-xs text-muted">
                      {([['Start', t.start_photo_id], ['Ende', t.end_photo_id]] as const).map(([label, fid]) => fid && (
                        <a key={label} href={fileUrl(vid, fid)} target="_blank" rel="noreferrer" className="flex items-center gap-2 hover:text-link">
                          <img src={fileUrl(vid, fid, 'thumbnail')} alt={`Tachofoto ${label}`} className="h-10 w-14 rounded-md border border-line object-cover" loading="lazy" />
                          Tacho {label}
                        </a>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </Card>
          <Card className="flex flex-col gap-3 p-5">
            <CardTitle>Im {monthName}</CardTitle>
            <div className="tabular font-display text-[28px] font-semibold">{report.data ? `${fmtNumber(report.data.total.value)} ${report.data.total.unit}` : '–'}</div>
            {report.data?.by_category.map((s) => (
              <div key={s.key} className="flex flex-col gap-1">
                <div className="flex justify-between text-sm"><span>{s.label}</span><span className="tabular font-semibold">{fmtNumber(s.distance.value)} {s.distance.unit} · {fmtNumber(s.share_pct)} %</span></div>
                <div className="h-2 rounded-full bg-soft"><div className="h-2 rounded-full bg-teal" style={{ width: `${Math.max(2, s.share_pct)}%` }} /></div>
              </div>
            ))}
            {report.data?.unassigned && report.data.unassigned.value > 0 && (
              <div className="text-xs text-muted">{fmtNumber(report.data.unassigned.value)} {report.data.unassigned.unit} laut Kilometerstand keiner Fahrt zugeordnet.</div>
            )}
          </Card>
        </div>
      </main>
      {dialog && <TripDialog vid={vehicle.id} mode={dialog} categories={cats.data?.items ?? []} onClose={() => setDialog(null)} />}
      {selected && <TripDialog vid={vehicle.id} mode="correct" trip={selected} categories={cats.data?.items ?? []} onClose={() => setSelected(null)} />}
    </>
  )
}

function useSaveHandler() {
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const onError = (e: unknown) => {
    if (e instanceof ProblemError && e.isPlausibility && e.problem.anomalies!.some((a) => a.confirmable)) setAnomalies(e.problem.anomalies!)
    else setError(errorText(e))
  }
  return { anomalies, setAnomalies, error, setError, onError }
}

function OpenTrip({ vid, trip, category, canEdit }: { vid: string; trip: Trip; category?: Category; canEdit: boolean }) {
  const qc = useQueryClient()
  const cur = useCurrent(vid).data
  const [km, setKm] = useState('')
  const [at, setAt] = useState(localInputValue())
  const [place, setPlace] = useState('')
  const h = useSaveHandler()
  const finish = useMutation({
    mutationFn: (extra: object) => api.post(`/vehicles/${vid}/trips/${trip.id}/finish`, {
      ended_at: new Date(at).toISOString(), end_odometer: { value: parseNumber(km) ?? 0, unit: 'km' }, end_location: place || null, ...extra,
    }, { 'If-Match': etagOf(trip) }),
    onSuccess: () => { ['trips', 'odometer'].forEach((k) => qc.invalidateQueries({ queryKey: [k, vid] })); setKm(''); h.setAnomalies(null) },
    onError: h.onError,
  })
  return (
    <section className="flex flex-col gap-4 rounded-[16px] bg-hero p-5 text-paper">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-ready text-ink"><Icon name="route" size={22} /></div>
        <div className="flex flex-col">
          <div className="font-display text-[19px] font-semibold">Fahrt läuft seit {fmtDate(trip.started_at)}</div>
          <div className="text-[13px] text-slate-300">Start bei {fmtNumber(trip.start_odometer.value)} km{trip.start_location ? ` · ${trip.start_location}` : ''}{category ? ` · ${category.name}` : ''}</div>
        </div>
      </div>
      {canEdit && (
        <form className="grid grid-cols-1 items-end gap-3 text-text sm:grid-cols-[1fr_1fr_1fr_auto]" onSubmit={(e) => { e.preventDefault(); h.setError(undefined); finish.mutate({}) }}>
          <input aria-label="Endstand" required inputMode="decimal" className={inputClass + ' tabular'} placeholder={cur?.meter_value ? `Endstand, zuletzt ${fmtNumber(cur.meter_value.canonical / 1000)}` : 'Endstand km'} value={km} onChange={(e) => setKm(e.target.value)} />
          <input aria-label="Ende" type="datetime-local" required className={inputClass} value={at} onChange={(e) => setAt(e.target.value)} />
          <input aria-label="Ziel" className={inputClass} placeholder="Ziel (optional)" value={place} onChange={(e) => setPlace(e.target.value)} />
          <Button type="submit" size="lg" disabled={finish.isPending}>Fahrt beenden</Button>
        </form>
      )}
      {h.error && <div role="alert" className="rounded-[10px] bg-card px-3 py-2 text-sm font-semibold text-bad">{h.error}</div>}
      {h.anomalies && <div className="text-text"><PlausibilityAlert anomalies={h.anomalies} busy={finish.isPending} onEdit={() => h.setAnomalies(null)}
        onConfirm={(codes, r) => finish.mutate({ confirm_anomalies: codes, anomaly_reason: r })} /></div>}
    </section>
  )
}

function TripDialog({ vid, mode, trip, categories, onClose }: { vid: string; mode: 'start' | 'record' | 'correct'; trip?: Trip; categories: Category[]; onClose: () => void }) {
  const qc = useQueryClient()
  const cur = useCurrent(vid).data
  const lastKm = cur?.meter_value ? String(Math.round(cur.meter_value.canonical / 1000)) : ''
  const active = categories.filter((c) => c.active !== false || c.id === trip?.category_id)
  const [f, setF] = useState({
    start: trip ? localInputValue(new Date(trip.started_at)) : localInputValue(), end: trip?.ended_at ? localInputValue(new Date(trip.ended_at)) : localInputValue(),
    startKm: trip ? String(trip.start_odometer.value) : lastKm, endKm: trip?.end_odometer ? String(trip.end_odometer.value) : '',
    from: trip?.start_location ?? '', to: trip?.end_location ?? '', purpose: trip?.purpose ?? '', category: trip?.category_id ?? active[0]?.id ?? '', reason: '',
  })
  const h = useSaveHandler()
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const cat = categories.find((c) => c.id === f.category)
  const done = () => { ['trips', 'odometer'].forEach((k) => qc.invalidateQueries({ queryKey: [k, vid] })); onClose() }
  const save = useMutation({
    mutationFn: (extra: object) => {
      const base = {
        started_at: new Date(f.start).toISOString(), time_zone: browserTimeZone(), start_odometer: { value: parseNumber(f.startKm) ?? 0, unit: 'km' },
        start_location: f.from || null, purpose: f.purpose || null, category_id: f.category, ...extra,
      }
      if (mode === 'start') return api.post(`/vehicles/${vid}/trips/start`, base)
      const full = { ...base, ended_at: new Date(f.end).toISOString(), end_odometer: { value: parseNumber(f.endKm) ?? 0, unit: 'km' }, end_location: f.to || null }
      if (mode === 'record') return api.post(`/vehicles/${vid}/trips`, full)
      return api.post(`/vehicles/${vid}/trips/${trip!.id}/corrections`, { ...full, reason: f.reason }, { 'If-Match': etagOf(trip!) })
    },
    onSuccess: done, onError: h.onError,
  })
  const cancel = useMutation({
    mutationFn: () => api.post(`/vehicles/${vid}/trips/${trip!.id}/cancel`, { reason: f.reason }, { 'If-Match': etagOf(trip!) }),
    onSuccess: done, onError: h.onError,
  })
  const title = { start: 'Fahrt starten', record: 'Fahrt nachtragen', correct: 'Fahrt korrigieren' }[mode]
  const dist = (parseNumber(f.endKm) ?? 0) - (parseNumber(f.startKm) ?? 0)

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={title}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); h.setError(undefined); h.setAnomalies(null); save.mutate({}) }}>
        {mode === 'correct' && <p className="m-0 text-sm leading-relaxed text-muted">Abgeschlossene Fahrten werden nicht überschrieben: Die Korrektur legt eine neue Fassung an, die alte bleibt in der Historie sichtbar.</p>}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Start" htmlFor="t-start"><input id="t-start" type="datetime-local" required className={inputClass} value={f.start} onChange={set('start')} /></Field>
          <Field label="Startstand (km)" htmlFor="t-skm"><input id="t-skm" inputMode="decimal" required className={inputClass + ' tabular'} value={f.startKm} onChange={set('startKm')} /></Field>
          {mode !== 'start' && <>
            <Field label="Ende" htmlFor="t-end"><input id="t-end" type="datetime-local" required className={inputClass} value={f.end} onChange={set('end')} /></Field>
            <Field label="Endstand (km)" htmlFor="t-ekm" hint={dist > 0 ? `${fmtNumber(dist)} km Strecke` : undefined}><input id="t-ekm" inputMode="decimal" required className={inputClass + ' tabular'} value={f.endKm} onChange={set('endKm')} /></Field>
          </>}
          <Field label="Von" htmlFor="t-from"><input id="t-from" className={inputClass} value={f.from} onChange={set('from')} /></Field>
          {mode !== 'start' && <Field label="Nach" htmlFor="t-to"><input id="t-to" className={inputClass} value={f.to} onChange={set('to')} /></Field>}
          <Field label="Kategorie" htmlFor="t-cat">
            <select id="t-cat" className={inputClass} value={f.category} onChange={set('category')}>{active.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select>
          </Field>
          <Field label={cat?.purpose_required ? 'Zweck (Pflicht)' : 'Zweck'} htmlFor="t-purpose"><input id="t-purpose" required={cat?.purpose_required} className={inputClass} value={f.purpose} onChange={set('purpose')} /></Field>
        </div>
        {mode === 'correct' && <Field label="Begründung" htmlFor="t-reason"><input id="t-reason" required className={inputClass} value={f.reason} onChange={set('reason')} placeholder="z. B. Tippfehler beim Endstand" /></Field>}
        {h.error && <div role="alert" className="text-sm font-semibold text-bad">{h.error}</div>}
        {h.anomalies && <PlausibilityAlert anomalies={h.anomalies} busy={save.isPending} onEdit={() => h.setAnomalies(null)}
          onConfirm={(codes, r) => save.mutate({ confirm_anomalies: codes, anomaly_reason: r })} />}
        {!h.anomalies && (
          <div className="flex flex-wrap justify-between gap-3">
            {mode === 'correct' ? <Button type="button" variant="outline" disabled={cancel.isPending || !f.reason.trim()} onClick={() => cancel.mutate()}>Stornieren</Button> : <span />}
            <Button type="submit" disabled={save.isPending || !f.category}>{mode === 'start' ? 'Fahrt starten' : 'Speichern'}</Button>
          </div>
        )}
      </form>
    </Dialog>
  )
}
