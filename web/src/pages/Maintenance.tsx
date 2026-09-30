import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { Button, Card, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { fmtNumber } from '../lib/format'
import { errorText, etagOf, fmtDay, parseNumber, today } from '../lib/money'
import { useCurrent } from '../lib/odometer'
import { useApp } from '../lib/state'

export type MaintItem = Schemas['MaintenanceItem'] & { version: number; status: Schemas['DueStatus'] }
type Due = Schemas['DueStatus']

export const levelInfo: Record<string, { label: string; tone: 'ok' | 'warn' | 'bad' | 'info' | 'neutral' }> = {
  overdue: { label: 'Überfällig', tone: 'bad' },
  due: { label: 'Fällig', tone: 'warn' },
  upcoming: { label: 'Demnächst', tone: 'info' },
  ok: { label: 'In Ordnung', tone: 'ok' },
  unknown: { label: 'Noch offen', tone: 'neutral' },
  completed: { label: 'Erledigt', tone: 'ok' },
}

const categoryLabel: Record<string, string> = {
  service: 'Inspektion/Service', legal_inspection: 'HU/AU', tires: 'Reifen', fluids: 'Flüssigkeiten', brakes: 'Bremsen', filters: 'Filter', other: 'Sonstiges',
}

export const maintKeys = (vid: string) => ['maintenance', vid]

export function useMaintenance(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: [...maintKeys(vid!), 'items'], queryFn: () => api.get<{ items: MaintItem[] }>(`/vehicles/${vid}/maintenance-items`) })
}

export function useDueStatus(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: [...maintKeys(vid!), 'status'], queryFn: () => api.get<{ items: Due[] }>(`/vehicles/${vid}/maintenance/status`) })
}

/** Kurzbeschreibung der Fälligkeit, z. B. „am 10.03.2027 · noch 1.300 km“. */
export function dueText(s: Due) {
  const parts: string[] = []
  if (s.due_date) parts.push(`${s.days_remaining != null && s.days_remaining < 0 ? 'seit' : 'am'} ${fmtDay(s.due_date)}`)
  if (s.distance_remaining) {
    const v = s.distance_remaining.value
    parts.push(v < 0 ? `${fmtNumber(-v)} ${s.distance_remaining.unit} drüber` : `noch ${fmtNumber(v)} ${s.distance_remaining.unit}`)
  } else if (s.due_total) parts.push(`bei ${fmtNumber(s.due_total.canonical / 1000)} km`)
  if (s.estimated && s.estimated_due_date) parts.push(`voraussichtlich ${fmtDay(s.estimated_due_date)}`)
  if (!parts.length && s.level === 'unknown') return 'Letzte Durchführung eintragen oder Kilometerstand erfassen'
  return parts.join(' · ')
}

function intervalText(i: MaintItem) {
  const p: string[] = []
  if (i.schedule_mode === 'once') {
    if (i.due_date_once) p.push(`einmalig bis ${fmtDay(i.due_date_once)}`)
    if (i.due_odometer_once) p.push(`bei ${fmtNumber(i.due_odometer_once.value)} ${i.due_odometer_once.unit}`)
    return p.join(' oder ')
  }
  if (i.interval_months) p.push(i.interval_months % 12 === 0 ? `${i.interval_months / 12} ${i.interval_months === 12 ? 'Jahr' : 'Jahre'}` : `${i.interval_months} Monate`)
  if (i.interval_days) p.push(`${i.interval_days} Tage`)
  if (i.interval_distance) p.push(`${fmtNumber(i.interval_distance.value)} ${i.interval_distance.unit}`)
  return 'alle ' + p.join(' oder ') + (i.schedule_mode === 'fixed_grid' ? ' (festes Raster)' : '')
}

export function MaintenancePage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const items = useMaintenance(vid)
  const [editing, setEditing] = useState<MaintItem | 'new' | null>(null)
  const [completing, setCompleting] = useState<MaintItem | null>(null)
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'

  if (!vehicle) return <><Header title="Wartung" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>

  const rank: Record<string, number> = { overdue: 5, due: 4, upcoming: 3, ok: 2, unknown: 1, completed: 0 }
  const list = [...(items.data?.items ?? [])].sort((a, b) => (rank[b.status.level] - rank[a.status.level]) ||
    ((a.status.estimated_due_date ?? '9999') < (b.status.estimated_due_date ?? '9999') ? -1 : 1))
  const counts = list.reduce<Record<string, number>>((m, i) => ({ ...m, [i.status.level]: (m[i.status.level] ?? 0) + 1 }), {})

  return (
    <>
      <Header title="Wartung" sub={`${vehicle.display_name} · was wann fällig wird`}
        actions={canEdit && <Button icon="plus" onClick={() => setEditing('new')}>Wartung planen</Button>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
          {(['overdue', 'due', 'upcoming', 'ok'] as const).map((l) => (
            <div key={l} className="flex flex-col gap-1 rounded-[16px] border border-line bg-card p-4">
              <Chip tone={levelInfo[l].tone}>{levelInfo[l].label}</Chip>
              <div className="tabular font-display text-[26px] font-semibold">{counts[l] ?? 0}</div>
            </div>
          ))}
        </div>
        {list.length === 0 && !items.isPending && (
          <EmptyState icon="wrench" title="Noch keine Wartungen geplant" text="Lege fest, was regelmäßig fällig ist – z. B. Ölwechsel alle 12 Monate oder 15.000 km, HU alle 24 Monate."
            action={canEdit && <Button icon="plus" onClick={() => setEditing('new')}>Wartung planen</Button>} />
        )}
        {list.length > 0 && (
          <Card className="overflow-hidden">
            {list.map((i, idx) => (
              <div key={i.id} className={`grid grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 px-5 py-3 ${idx ? 'border-t border-line' : ''} ${i.active === false ? 'opacity-60' : ''}`}>
                <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name="wrench" /></div>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[15px] font-semibold">{i.title}</span>
                    <Chip tone={levelInfo[i.status.level].tone}>{levelInfo[i.status.level].label}</Chip>
                    {i.active === false && <Chip>pausiert</Chip>}
                  </div>
                  <div className="text-xs text-muted">{categoryLabel[i.category]} · {intervalText(i)}</div>
                  <div className="text-[13px]">{dueText(i.status)}</div>
                </div>
                {canEdit && (
                  <div className="flex items-center gap-2">
                    {i.status.level !== 'completed' && <Button variant="outline" icon="check" onClick={() => setCompleting(i)}>Erledigt</Button>}
                    <button type="button" aria-label="Bearbeiten" onClick={() => setEditing(i)} className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="edit" size={18} /></button>
                  </div>
                )}
              </div>
            ))}
          </Card>
        )}
        <div className="flex items-start gap-2.5 rounded-[12px] bg-info-bg px-3.5 py-3 text-[13px] leading-normal">
          <span className="text-link"><Icon name="info" size={18} /></span>
          <div>Fälligkeiten werden immer neu berechnet – aus dem Intervall, der letzten Erledigung und dem aktuellen Kilometerstand. Wer einen Serviceeintrag erfasst, kann dort direkt die erledigten Wartungen abhaken.</div>
        </div>
      </main>
      {editing && <ItemDialog vid={vehicle.id} item={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
      {completing && <CompleteDialog vid={vehicle.id} item={completing} onClose={() => setCompleting(null)} />}
    </>
  )
}

interface ItemForm {
  title: string; category: string; mode: string; months: string; km: string; anchorDate: string; anchorKm: string; onceDate: string; onceKm: string; note: string; active: boolean
}

function ItemDialog({ vid, item, onClose }: { vid: string; item: MaintItem | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<ItemForm>({
    title: item?.title ?? '', category: item?.category ?? 'service', mode: item?.schedule_mode ?? 'from_last_completion',
    months: item?.interval_months ? String(item.interval_months) : '', km: item?.interval_distance ? String(item.interval_distance.value) : '',
    anchorDate: item?.anchor_date ?? '', anchorKm: item?.anchor_odometer ? String(item.anchor_odometer.value) : '',
    onceDate: item?.due_date_once ?? '', onceKm: item?.due_odometer_once ? String(item.due_odometer_once.value) : '', note: item?.note ?? '',
    active: item?.active !== false,
  })
  const [error, setError] = useState<string>()
  const set = (k: keyof ItemForm) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const q = (v: string) => { const n = parseNumber(v); return n === null ? null : { value: n, unit: 'km' } }
  const done = () => { qc.invalidateQueries({ queryKey: ['maintenance', vid] }); onClose() }
  const save = useMutation({
    mutationFn: () => {
      const once = f.mode === 'once'
      const body = {
        title: f.title, category: f.category, schedule_mode: f.mode, note: f.note, active: f.active,
        interval_months: once ? null : parseNumber(f.months), interval_days: null, interval_distance: once ? null : q(f.km),
        anchor_date: once ? null : f.anchorDate || null, anchor_odometer: once ? null : q(f.anchorKm),
        due_date_once: once ? f.onceDate || null : null, due_odometer_once: once ? q(f.onceKm) : null,
      }
      return item ? api.patch(`/vehicles/${vid}/maintenance-items/${item.id}`, body, etagOf(item)) : api.post(`/vehicles/${vid}/maintenance-items`, body)
    },
    onSuccess: done, onError: (e) => setError(errorText(e)),
  })
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vid}/maintenance-items/${item!.id}`, etagOf(item!)), onSuccess: done, onError: (e) => setError(errorText(e)) })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={item ? 'Wartung bearbeiten' : 'Wartung planen'}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); save.mutate() }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Bezeichnung" htmlFor="m-title"><input id="m-title" required className={inputClass} value={f.title} onChange={set('title')} placeholder="Ölwechsel" /></Field>
          <Field label="Kategorie" htmlFor="m-cat">
            <select id="m-cat" className={inputClass} value={f.category} onChange={set('category')}>
              {Object.entries(categoryLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}
            </select>
          </Field>
        </div>
        <Field label="Planung" htmlFor="m-mode">
          <select id="m-mode" className={inputClass} value={f.mode} onChange={set('mode')}>
            <option value="from_last_completion">Wiederkehrend ab letzter Erledigung</option>
            <option value="fixed_grid">Wiederkehrend im festen Raster</option>
            <option value="once">Einmalig</option>
          </select>
        </Field>
        {f.mode === 'once' ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Fällig am" htmlFor="m-od"><input id="m-od" type="date" className={inputClass} value={f.onceDate} onChange={set('onceDate')} /></Field>
            <Field label="oder bei km" htmlFor="m-ok"><input id="m-ok" inputMode="decimal" className={inputClass} value={f.onceKm} onChange={set('onceKm')} /></Field>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Alle … Monate" htmlFor="m-months"><input id="m-months" inputMode="numeric" className={inputClass} value={f.months} onChange={set('months')} placeholder="12" /></Field>
              <Field label="oder alle … km" htmlFor="m-km"><input id="m-km" inputMode="decimal" className={inputClass} value={f.km} onChange={set('km')} placeholder="15000" /></Field>
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label={f.mode === 'fixed_grid' ? 'Rasterbeginn (Datum)' : 'Zuletzt gemacht am'} htmlFor="m-ad" hint="Nur nötig, solange es keine Erledigung gibt">
                <input id="m-ad" type="date" className={inputClass} value={f.anchorDate} onChange={set('anchorDate')} />
              </Field>
              <Field label={f.mode === 'fixed_grid' ? 'Rasterbeginn (km)' : 'bei km'} htmlFor="m-ak"><input id="m-ak" inputMode="decimal" className={inputClass} value={f.anchorKm} onChange={set('anchorKm')} /></Field>
            </div>
          </>
        )}
        <Field label="Notiz" htmlFor="m-note"><input id="m-note" className={inputClass} value={f.note} onChange={set('note')} /></Field>
        {item && (
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" className="h-4 w-4 accent-teal" checked={f.active} onChange={(e) => setF({ ...f, active: e.target.checked })} />Aktiv (wird bewertet)
          </label>
        )}
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <div className="flex flex-wrap justify-between gap-3">
          {item ? <Button type="button" variant="outline" disabled={remove.isPending} onClick={() => remove.mutate()}>Löschen</Button> : <span />}
          <Button type="submit" disabled={save.isPending || !f.title.trim()}>Speichern</Button>
        </div>
      </form>
    </Dialog>
  )
}

function CompleteDialog({ vid, item, onClose }: { vid: string; item: MaintItem; onClose: () => void }) {
  const qc = useQueryClient()
  const cur = useCurrent(vid).data
  const [kind, setKind] = useState<'done' | 'skipped'>('done')
  const [date, setDate] = useState(today())
  const [km, setKm] = useState(cur?.meter_value ? String(Math.round(cur.meter_value.canonical / 1000)) : '')
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string>()
  const [key] = useState(() => crypto.randomUUID())
  const save = useMutation({
    mutationFn: () => {
      const n = parseNumber(km)
      return api.post(`/vehicles/${vid}/maintenance-items/${item.id}/completions`, {
        kind, completed_on: date, completed_odometer: n === null ? null : { value: n, unit: 'km' }, reason: reason || null,
      }, { 'Idempotency-Key': key })
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['maintenance', vid] }); onClose() },
    onError: (e) => setError(errorText(e)),
  })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={`${item.title} erledigen`}>
      <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); setError(undefined); save.mutate() }}>
        <p className="m-0 text-sm leading-relaxed text-muted">Für Werkstattbesuche mit Rechnung besser einen Serviceeintrag anlegen – dort lässt sich die Wartung mit abhaken.</p>
        <div className="flex gap-2">
          {(['done', 'skipped'] as const).map((k) => (
            <button key={k} type="button" onClick={() => setKind(k)} aria-pressed={kind === k}
              className={`h-10 rounded-[10px] border px-4 text-sm font-semibold ${kind === k ? 'border-teal bg-info-bg text-link' : 'border-line'}`}>
              {k === 'done' ? 'Erledigt' : 'Ausgelassen'}
            </button>
          ))}
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Datum" htmlFor="c-date"><input id="c-date" type="date" required max={today()} className={inputClass} value={date} onChange={(e) => setDate(e.target.value)} /></Field>
          <Field label="Kilometerstand" htmlFor="c-km" hint="optional"><input id="c-km" inputMode="decimal" className={inputClass} value={km} onChange={(e) => setKm(e.target.value)} /></Field>
        </div>
        <Field label={kind === 'skipped' ? 'Begründung (Pflicht)' : 'Notiz'} htmlFor="c-reason">
          <input id="c-reason" className={inputClass} value={reason} onChange={(e) => setReason(e.target.value)} required={kind === 'skipped'} />
        </Field>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <Button type="submit" className="self-end" disabled={save.isPending}>Speichern</Button>
      </form>
    </Dialog>
  )
}
