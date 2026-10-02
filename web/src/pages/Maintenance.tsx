import { useMemo, useState, type FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { Button, Card, Chip, EmptyState, Field, Segmented, inputClass } from '../components/ui'
import { fmtDate, fmtNumber, num, today } from '../lib/format'
import { dueText, levelInfo } from '../lib/due'
import { problemText } from '../lib/mutation'
import { useCurrent } from '../lib/odometer'
import { useApp, type Vehicle } from '../lib/state'

type Item = Schemas['MaintenanceItem']
type Template = Schemas['MaintenanceTemplate']

export const maintKeys = (vid: string) => ['maintenance', vid]

const categoryLabel: Record<string, string> = { service: 'Wartung', legal_inspection: 'Prüfung (HU/AU)', tires: 'Reifen', fluids: 'Flüssigkeiten', brakes: 'Bremsen', filters: 'Filter', other: 'Sonstiges' }

function intervalText(i: Item) {
  if (i.schedule_mode === 'once') return ['einmalig', i.due_date_once && `bis ${fmtDate(i.due_date_once, true)}`, i.due_odometer_once && `bei ${fmtNumber(i.due_odometer_once.value)} ${i.due_odometer_once.unit}`].filter(Boolean).join(' ')
  const p = [i.interval_months && `${i.interval_months} Monate`, i.interval_days && `${i.interval_days} Tage`, i.interval_distance && `${fmtNumber(i.interval_distance.value)} ${i.interval_distance.unit}`].filter(Boolean)
  return `alle ${p.join(' oder ')}${i.schedule_mode === 'fixed_grid' ? ' (festes Raster)' : ''}${!i.anchor_date && !i.anchor_odometer ? ' · Start: Anlegedatum' : ''}`
}

export function MaintenancePage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const items = useQuery({ enabled: !!vid, queryKey: [...maintKeys(vid!), 'items'], queryFn: () => api.get<Schemas['MaintenanceItemPage']>(`/vehicles/${vid}/maintenance-items`) })
  const [dialog, setDialog] = useState<'new' | 'template' | null>(null)
  const [edit, setEdit] = useState<Item | null>(null)
  const [complete, setComplete] = useState<Item | null>(null)
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'

  if (!vehicle) return <><Header title="Wartung" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>
  const list = items.data?.items ?? []
  const counts = list.reduce<Record<string, number>>((a, i) => { const l = i.status?.level ?? 'unknown'; a[l] = (a[l] ?? 0) + 1; return a }, {})

  return (
    <>
      <Header title="Wartung" sub={`${vehicle.display_name} · ${list.length} Positionen${counts.overdue ? ` · ${counts.overdue} überfällig` : ''}${counts.due ? ` · ${counts.due} fällig` : ''}`}
        actions={canEdit && <div className="flex gap-2">
          <Button variant="outline" icon="doc" onClick={() => setDialog('template')}>Wartungsplan-Vorlage</Button>
          <Button icon="plus" onClick={() => setDialog('new')}>Wartung anlegen</Button>
        </div>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        {list.length === 0 && !items.isPending && (
          <EmptyState icon="wrench" title="Noch keine Wartungen" text="Übernimm einen Wartungsplan aus einer Vorlage (z. B. Mercedes-Benz Sprinter oder Vito CDI, Modelljahr 2019) und trage ein, wann die Arbeiten zuletzt gemacht wurden."
            action={canEdit && <Button icon="doc" onClick={() => setDialog('template')}>Vorlage auswählen</Button>} />
        )}
        {list.length > 0 && (
          <Card className="flex flex-col overflow-hidden">
            {list.map((i) => {
              const li = levelInfo[i.status?.level ?? 'unknown']
              return (
                <div key={i.id} className={`grid min-h-[68px] grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-2.5 first:border-t-0 ${i.active === false ? 'opacity-60' : ''}`}>
                  <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name={i.category === 'legal_inspection' ? 'check' : i.category === 'tires' ? 'sync' : 'wrench'} /></div>
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <div className="flex flex-wrap items-center gap-2 text-[15px] font-semibold">{i.title}<Chip tone={li.tone}>{li.label}</Chip>{i.active === false && <Chip>inaktiv</Chip>}</div>
                    <div className="text-xs text-muted">{categoryLabel[i.category]} · {intervalText(i)}{i.status && i.active !== false ? ` · ${dueText(i.status)}` : ''}{i.status?.estimated ? ' (geschätzt)' : ''}</div>
                  </div>
                  {canEdit && (
                    <div className="flex items-center gap-1.5">
                      {i.status?.level !== 'completed' && <Button variant="outline" icon="check" onClick={() => setComplete(i)}>Erledigt</Button>}
                      <button type="button" onClick={() => setEdit(i)} aria-label="Bearbeiten" className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="edit" size={18} /></button>
                    </div>
                  )}
                </div>
              )
            })}
          </Card>
        )}
      </main>
      {dialog === 'new' && <ItemDialog vehicle={vehicle} onClose={() => setDialog(null)} />}
      {edit && <ItemDialog vehicle={vehicle} item={edit} onClose={() => setEdit(null)} />}
      {dialog === 'template' && <TemplateDialog vehicle={vehicle} onClose={() => setDialog(null)} />}
      {complete && <CompleteDialog vid={vehicle.id} item={complete} onClose={() => setComplete(null)} />}
    </>
  )
}

interface ItemForm {
  title: string; category: string; mode: string; months: string; km: string; anchorDate: string; anchorKm: string; dueDate: string; dueKm: string
  upcomingDays: string; upcomingKm: string; active: boolean; description: string
}

function formOf(i?: Item): ItemForm {
  return { title: i?.title ?? '', category: i?.category ?? 'service', mode: i?.schedule_mode ?? 'from_last_completion', months: i?.interval_months?.toString() ?? '',
    km: i?.interval_distance ? String(i.interval_distance.value) : '', anchorDate: i?.anchor_date ?? '', anchorKm: i?.anchor_odometer ? String(i.anchor_odometer.value) : '',
    dueDate: i?.due_date_once ?? '', dueKm: i?.due_odometer_once ? String(i.due_odometer_once.value) : '',
    upcomingDays: i?.thresholds?.upcoming_days?.toString() ?? '', upcomingKm: i?.thresholds?.upcoming_distance ? String(i.thresholds.upcoming_distance.value) : '',
    active: i?.active ?? true, description: i?.description ?? '' }
}

function bodyOf(f: ItemForm) {
  const q = (v: string) => (v ? { value: num(v), unit: 'km' } : null)
  const once = f.mode === 'once'
  return {
    title: f.title, category: f.category, schedule_mode: f.mode, description: f.description || null, active: f.active,
    interval_months: !once && f.months ? Number(f.months) : null, interval_distance: once ? null : q(f.km),
    anchor_date: f.anchorDate || null, anchor_odometer: q(f.anchorKm),
    due_date_once: once ? f.dueDate || null : null, due_odometer_once: once ? q(f.dueKm) : null,
    thresholds: { upcoming_days: f.upcomingDays ? Number(f.upcomingDays) : null, upcoming_distance: q(f.upcomingKm) },
  }
}

function ItemDialog({ vehicle, item, onClose }: { vehicle: Vehicle; item?: Item; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState(() => formOf(item))
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const set = (k: keyof ItemForm) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const base = `/vehicles/${vehicle.id}/maintenance-items`
  async function run(fn: () => Promise<unknown>) {
    setBusy(true)
    setError(undefined)
    try { await fn(); qc.invalidateQueries({ queryKey: maintKeys(vehicle.id) }); qc.invalidateQueries({ queryKey: ['due'] }); onClose() } catch (e) { setError(problemText(e)) } finally { setBusy(false) }
  }
  function submit(e: FormEvent) {
    e.preventDefault()
    const body = bodyOf(f)
    run(() => (item ? api.patch(`${base}/${item.id}`, body, `"${item.version}"`) : api.post(base, body)))
  }
  const once = f.mode === 'once'
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={item ? 'Wartung bearbeiten' : 'Wartung anlegen'}>
      <form className="flex flex-col gap-4" onSubmit={submit}>
        <Field label="Bezeichnung" htmlFor="m-title"><input id="m-title" required className={inputClass} value={f.title} onChange={set('title')} placeholder="Ölwechsel" /></Field>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Kategorie" htmlFor="m-cat">
            <select id="m-cat" className={inputClass} value={f.category} onChange={set('category')}>{Object.entries(categoryLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select>
          </Field>
          <Field label="Planung" htmlFor="m-mode">
            <select id="m-mode" className={inputClass} value={f.mode} onChange={set('mode')}>
              <option value="from_last_completion">ab letzter Erledigung</option><option value="fixed_grid">festes Raster ab Start</option><option value="once">einmalig</option>
            </select>
          </Field>
          {!once && <>
            <Field label="Intervall (Monate)" htmlFor="m-months"><input id="m-months" inputMode="numeric" className={inputClass} value={f.months} onChange={set('months')} /></Field>
            <Field label="Intervall (km)" htmlFor="m-km"><input id="m-km" inputMode="decimal" className={inputClass} value={f.km} onChange={set('km')} /></Field>
            <Field label="Zuletzt erledigt am / Start" htmlFor="m-ad" hint="Ohne Datum zählt das Anlegedatum"><input id="m-ad" type="date" className={inputClass} value={f.anchorDate} onChange={set('anchorDate')} /></Field>
            <Field label="… bei km-Stand" htmlFor="m-ak"><input id="m-ak" inputMode="decimal" className={inputClass} value={f.anchorKm} onChange={set('anchorKm')} /></Field>
          </>}
          {once && <>
            <Field label="Fällig am" htmlFor="m-dd"><input id="m-dd" type="date" className={inputClass} value={f.dueDate} onChange={set('dueDate')} /></Field>
            <Field label="Fällig bei km" htmlFor="m-dk"><input id="m-dk" inputMode="decimal" className={inputClass} value={f.dueKm} onChange={set('dueKm')} /></Field>
          </>}
          <Field label="„Demnächst“ ab (Tage vorher)" htmlFor="m-ud" hint="leer = Vorgabe aus den Einstellungen"><input id="m-ud" inputMode="numeric" className={inputClass} value={f.upcomingDays} onChange={set('upcomingDays')} /></Field>
          <Field label="„Demnächst“ ab (km vorher)" htmlFor="m-uk"><input id="m-uk" inputMode="decimal" className={inputClass} value={f.upcomingKm} onChange={set('upcomingKm')} /></Field>
        </div>
        <Field label="Beschreibung" htmlFor="m-desc"><input id="m-desc" className={inputClass} value={f.description} onChange={set('description')} /></Field>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="h-4 w-4 accent-teal" checked={f.active} onChange={(e) => setF({ ...f, active: e.target.checked })} />aktiv (wird bewertet)</label>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <div className="flex flex-wrap justify-between gap-3">
          {item ? <Button type="button" variant="outline" disabled={busy} onClick={() => confirm('Wartung löschen?') && run(() => api.del(`${base}/${item.id}`, `"${item.version}"`))}>Löschen</Button> : <span />}
          <Button type="submit" disabled={busy}>Speichern</Button>
        </div>
      </form>
    </Dialog>
  )
}

function CompleteDialog({ vid, item, onClose }: { vid: string; item: Item; onClose: () => void }) {
  const qc = useQueryClient()
  const cur = useCurrent(vid).data
  const [kind, setKind] = useState<'done' | 'skipped'>('done')
  const [on, setOn] = useState(today())
  const [km, setKm] = useState(cur?.meter_value ? String(cur.meter_value.canonical / 1000) : '')
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [key] = useState(() => crypto.randomUUID())
  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(undefined)
    try {
      await api.post(`/vehicles/${vid}/maintenance-items/${item.id}/completions`,
        { kind, completed_on: on, completed_odometer: km ? { value: num(km), unit: 'km' } : null, reason: reason || null }, { 'Idempotency-Key': key })
      qc.invalidateQueries({ queryKey: maintKeys(vid) })
      qc.invalidateQueries({ queryKey: ['due'] })
      onClose()
    } catch (err) { setError(err instanceof ProblemError ? problemText(err) : 'Fehlgeschlagen.') } finally { setBusy(false) }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={item.title}>
      <form className="flex flex-col gap-4" onSubmit={submit}>
        <Segmented label="Art" value={kind} onChange={setKind} options={[['done', 'Erledigt'], ['skipped', 'Bewusst ausgelassen']]} />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Datum" htmlFor="c-on"><input id="c-on" type="date" required className={inputClass} value={on} onChange={(e) => setOn(e.target.value)} /></Field>
          <Field label="Kilometerstand" htmlFor="c-km" hint="leer = aus den Kilometerständen"><input id="c-km" inputMode="decimal" className={inputClass} value={km} onChange={(e) => setKm(e.target.value)} /></Field>
        </div>
        <Field label={kind === 'skipped' ? 'Begründung (Pflicht)' : 'Notiz'} htmlFor="c-reason"><input id="c-reason" required={kind === 'skipped'} className={inputClass} value={reason} onChange={(e) => setReason(e.target.value)} /></Field>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <Button type="submit" disabled={busy} className="self-end">Speichern</Button>
      </form>
    </Dialog>
  )
}

interface Row { include: boolean; title: string; months: string; km: string; lastDate: string; lastKm: string }

function TemplateDialog({ vehicle, onClose }: { vehicle: Vehicle; onClose: () => void }) {
  const qc = useQueryClient()
  const templates = useQuery({ queryKey: ['maintenance-templates'], queryFn: () => api.get<{ items: Template[] }>('/maintenance-templates') })
  const sorted = useMemo(() => {
    const all = templates.data?.items ?? []
    const score = (t: Template) => (t.body_types?.includes(vehicle.body_type) ? 2 : 0) + (t.energy_carriers?.some((c) => vehicle.energy_carriers.includes(c as never)) ? 1 : 0)
    return [...all].sort((a, b) => score(b) - score(a))
  }, [templates.data, vehicle])
  const [tid, setTid] = useState<string>()
  const t = sorted.find((x) => x.id === tid) ?? sorted[0]
  const [rows, setRows] = useState<Record<string, Row>>({})
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<string>()
  const [error, setError] = useState<string>()
  const existing = useQuery({ queryKey: [...maintKeys(vehicle.id), 'items'], queryFn: () => api.get<Schemas['MaintenanceItemPage']>(`/vehicles/${vehicle.id}/maintenance-items`) })
  const have = new Set((existing.data?.items ?? []).map((i) => i.title))

  const rowOf = (it: Template['items'][number]): Row => rows[`${t?.id}:${it.key}`] ?? { include: !it.optional && !have.has(it.title), title: it.title,
    months: it.interval_months?.toString() ?? '', km: it.interval_km?.toString() ?? '', lastDate: '', lastKm: '' }
  const setRow = (key: string, r: Row) => setRows({ ...rows, [`${t?.id}:${key}`]: r })

  async function apply() {
    if (!t) return
    setBusy(true)
    setError(undefined)
    let n = 0
    const errors: string[] = []
    for (const it of t.items) {
      const r = rowOf(it)
      if (!r.include) continue
      const body = {
        title: r.title, category: it.category, schedule_mode: it.schedule_mode, description: [it.description, `Vorlage: ${t.title}`].filter(Boolean).join(' – '),
        manufacturer_recommended: false, interval_months: r.months ? Number(r.months) : null,
        interval_distance: r.km ? { value: num(r.km), unit: 'km' } : null,
        anchor_date: r.lastDate || null, anchor_odometer: r.lastKm ? { value: num(r.lastKm), unit: 'km' } : null,
      }
      try { await api.post(`/vehicles/${vehicle.id}/maintenance-items`, body); n++ } catch (e) { errors.push(`${r.title}: ${problemText(e)}`) }
    }
    setBusy(false)
    qc.invalidateQueries({ queryKey: maintKeys(vehicle.id) })
    qc.invalidateQueries({ queryKey: ['due'] })
    if (errors.length) setError(errors.join('\n'))
    else { setResult(`${n} Wartungen übernommen.`); setTimeout(onClose, 900) }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title="Wartungsplan aus Vorlage" wide>
      {!t && <div className="text-sm text-muted">Lade Vorlagen …</div>}
      {t && (
        <div className="flex flex-col gap-4">
          <Field label="Vorlage" htmlFor="t-sel">
            <select id="t-sel" className={inputClass} value={t.id} onChange={(e) => setTid(e.target.value)}>
              {sorted.map((x) => <option key={x.id} value={x.id}>{x.title}</option>)}
            </select>
          </Field>
          <div className="text-[13px] leading-relaxed text-muted">{t.description} {t.applies_to && <>Gilt für: {t.applies_to}.</>}</div>
          {t.note && <div className="flex gap-2 rounded-[12px] bg-warn-bg px-3 py-2.5 text-[13px] text-warn"><Icon name="info" size={18} />{t.note}</div>}
          <div className="text-[13px]">Trage für jede Position ein, wann sie <b>zuletzt erledigt</b> wurde (Datum und/oder km). Daraus berechnet Vectra die nächste Fälligkeit. Intervalle kannst du anpassen.</div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] border-collapse text-sm">
              <thead><tr className="text-left text-xs text-muted">
                <th className="py-1.5 pr-2 font-semibold">Übernehmen</th><th className="py-1.5 pr-2 font-semibold">Position</th><th className="py-1.5 pr-2 font-semibold">Monate</th>
                <th className="py-1.5 pr-2 font-semibold">km</th><th className="py-1.5 pr-2 font-semibold">zuletzt am</th><th className="py-1.5 font-semibold">bei km</th>
              </tr></thead>
              <tbody>
                {t.items.map((it) => {
                  const r = rowOf(it)
                  const cell = 'h-9 w-full rounded-[9px] border border-line bg-card px-2 text-sm'
                  return (
                    <tr key={it.key} className="border-t border-line align-middle">
                      <td className="py-2 pr-2"><input type="checkbox" aria-label={`${it.title} übernehmen`} className="h-4 w-4 accent-teal" checked={r.include} onChange={(e) => setRow(it.key, { ...r, include: e.target.checked })} /></td>
                      <td className="py-2 pr-2">
                        <input aria-label="Bezeichnung" className={cell + ' min-w-[220px] font-semibold'} value={r.title} onChange={(e) => setRow(it.key, { ...r, title: e.target.value })} />
                        <div className="mt-0.5 text-xs text-muted">{[categoryLabel[it.category], it.optional && 'optional', have.has(it.title) && 'schon vorhanden', it.description].filter(Boolean).join(' · ')}</div>
                      </td>
                      <td className="w-20 py-2 pr-2"><input aria-label="Monate" inputMode="numeric" className={cell} value={r.months} onChange={(e) => setRow(it.key, { ...r, months: e.target.value })} /></td>
                      <td className="w-24 py-2 pr-2"><input aria-label="km" inputMode="decimal" className={cell} value={r.km} onChange={(e) => setRow(it.key, { ...r, km: e.target.value })} /></td>
                      <td className="w-36 py-2 pr-2"><input aria-label="zuletzt am" type="date" className={cell} value={r.lastDate} onChange={(e) => setRow(it.key, { ...r, lastDate: e.target.value })} /></td>
                      <td className="w-28 py-2"><input aria-label="bei km" inputMode="decimal" className={cell} value={r.lastKm} onChange={(e) => setRow(it.key, { ...r, lastKm: e.target.value })} /></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          {error && <div role="alert" className="text-sm font-semibold whitespace-pre-line text-bad">{error}</div>}
          {result && <div role="status" className="text-sm font-semibold text-ok">{result}</div>}
          <Button type="button" icon="check" disabled={busy || !t.items.some((it) => rowOf(it).include)} onClick={apply} className="self-end">Ausgewählte übernehmen</Button>
        </div>
      )}
    </Dialog>
  )
}
