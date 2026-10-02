import { useState, type FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Card, CardTitle, Chip, EmptyState, Field, Kpi, Segmented, inputClass } from '../components/ui'
import { browserTimeZone, fmtDate, fmtDisplay, fmtNumber, localInputValue, num, toIso } from '../lib/format'
import { usePlausibleMutation } from '../lib/mutation'
import { useCurrent } from '../lib/odometer'
import { useApp } from '../lib/state'

type Entry = Schemas['OilEntry']
type Kind = 'check' | 'top_up' | 'oil_change'

export const oilKeys = (vid: string) => ['oil', vid]

const kindLabel: Record<Kind, string> = { check: 'Messung', top_up: 'Nachfüllung', oil_change: 'Ölwechsel' }
const steps: [string, string][] = [['', 'Prozent'], ['min', 'Min'], ['quarter', '¼'], ['half', '½'], ['three_quarters', '¾'], ['max', 'Max'], ['below_min', 'unter Min'], ['above_max', 'über Max']]
const pairReason: Record<string, string> = { no_range: 'Peilstab-Spanne fehlt', no_level: 'kein vergleichbarer Stand', non_positive_distance: 'keine Strecke', series_start: 'Beginn der Messreihe' }

interface Draft {
  kind: Kind; at: string; dateOnly: boolean; odometer: string; beforeStep: string; before: string; afterStep: string; after: string
  added: string; addedUnit: string; fill: string; spec: string; brand: string; filter: boolean; note: string
}

const empty = (spec = ''): Draft => ({ kind: 'check', at: localInputValue(), dateOnly: false, odometer: '', beforeStep: '', before: '', afterStep: '', after: '',
  added: '', addedUnit: 'l', fill: '', spec, brand: '', filter: true, note: '' })

function yearAgo() {
  const d = new Date()
  d.setFullYear(d.getFullYear() - 1)
  return localInputValue(d).slice(0, 10)
}

function level(step: string, pct: string) {
  if (step) return { step }
  if (pct.trim() === '') return undefined
  return { percent: num(pct) }
}

function bodyOf(d: Draft) {
  const b: Record<string, unknown> = { kind: d.kind, occurred_at: toIso(d.at, d.dateOnly), time_zone: browserTimeZone(), time_precision: d.dateOnly ? 'date_only' : 'exact', note: d.note }
  if (d.odometer) b.odometer = { value: num(d.odometer), unit: 'km' }
  const before = level(d.beforeStep, d.before)
  const after = level(d.afterStep, d.after)
  if (before && d.kind !== 'oil_change') b.level_before = before
  if (after && d.kind !== 'check') b.level_after = after
  if (d.kind === 'top_up' && d.added) b.oil_added = { value: num(d.added), unit: d.addedUnit }
  if (d.kind === 'oil_change') {
    if (d.fill) b.oil_change_fill = { value: num(d.fill), unit: 'l' }
    b.filter_changed = d.filter
  }
  if (d.spec) b.oil_specification = d.spec
  if (d.brand) b.oil_brand = d.brand
  return b
}

function LevelInput({ id, label, step, pct, onStep, onPct }: { id: string; label: string; step: string; pct: string; onStep: (v: string) => void; onPct: (v: string) => void }) {
  return (
    <Field label={label} htmlFor={id} hint="0 % = Min-Markierung, 100 % = Max-Markierung">
      <div className="flex gap-2">
        <select aria-label={label + ' Stufe'} className={inputClass + ' w-auto'} value={step} onChange={(e) => onStep(e.target.value)}>
          {steps.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
        </select>
        {!step && <input id={id} inputMode="decimal" className={inputClass + ' tabular'} value={pct} onChange={(e) => onPct(e.target.value)} placeholder="z. B. 60" />}
      </div>
    </Field>
  )
}

export function OilPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const qc = useQueryClient()
  const cur = useCurrent(vid).data
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'
  const entries = useQuery({ enabled: !!vid, queryKey: [...oilKeys(vid!), 'entries'], queryFn: () => api.get<{ items: Entry[] }>(`/vehicles/${vid}/oil-entries?limit=100`) })
  const series = useQuery({ enabled: !!vid, queryKey: [...oilKeys(vid!), 'series'], queryFn: () => api.get<Schemas['OilSeriesPage']>(`/vehicles/${vid}/oil/series`) })
  const stats = useQuery({ enabled: !!vid, queryKey: [...oilKeys(vid!), 'stats'], queryFn: () => api.get<Schemas['OilStatistics']>(`/vehicles/${vid}/oil/statistics?from=${yearAgo()}`) })
  const lastSpec = entries.data?.items.find((e) => e.oil_specification)?.oil_specification ?? ''
  const [draft, setDraft] = useState<Draft | null>(null)
  const d = draft ?? empty(lastSpec)
  const save = usePlausibleMutation(
    (extra) => api.post(`/vehicles/${vid}/oil-entries`, { ...bodyOf(d), ...extra }),
    () => { setDraft(empty(d.spec)); qc.invalidateQueries({ queryKey: oilKeys(vid!) }); qc.invalidateQueries({ queryKey: ['odometer', vid] }) },
  )

  if (!vehicle) return <><Header title="Öl" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>

  const upd = (p: Partial<Draft>) => setDraft({ ...d, ...p })
  const items = entries.data?.items ?? []
  const current = series.data?.items[0]
  const st = stats.data
  const noRange = !vehicle.oil_dipstick_range

  function submit(e: FormEvent) {
    e.preventDefault()
    save.submit()
  }

  return (
    <>
      <Header title="Öl" sub={`${vehicle.display_name} · Ölstand, Nachfüllung, Ölwechsel`} />
      <main className="grid min-h-0 flex-grow grid-cols-1 items-start gap-5 overflow-y-auto px-8 py-6 xl:grid-cols-2">
        <div className="flex flex-col gap-5">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Kpi icon="oil" label="Verbrauch seit Ölwechsel" value={current?.status === 'ok' ? fmtDisplay(current.consumption_per_1000, 0) : '–'}
              note={current?.status === 'too_few_measurements' ? 'zu wenige Messungen (mindestens 2 nötig)' : current?.distance_since_start ? `seit ${fmtDisplay(current.distance_since_start, 0)}` : 'aus Ölstandsmessungen'} />
            <Kpi icon="trend" label="Nachfüllrate (12 Monate)" value={fmtDisplay(st?.top_up_rate, 0)}
              note={st?.top_up_rate_unavailable_reason === 'distance_unknown' ? 'Strecke unbekannt' : st?.top_up_rate_unavailable_reason === 'distance_zero' ? 'keine Strecke gefahren' : 'was nachgefüllt wurde'} />
            <Kpi icon="plus" label="Nachgefüllt" value={st ? fmtDisplay(st.total_added, 2) : '–'} note={st ? `${st.top_up_count}× nachgefüllt, ${st.oil_change_count}× gewechselt` : ''} />
          </div>
          {st?.hints.includes('consumption_increase') && (
            <div role="status" className="flex gap-2.5 rounded-[12px] bg-warn-bg px-3.5 py-3 text-[13px] text-warn"><Icon name="alert" size={18} />Der Verbrauch seit dem letzten Ölwechsel liegt deutlich über den vorigen Messreihen.</div>
          )}
          {noRange && (
            <div className="flex gap-2.5 rounded-[12px] bg-info-bg px-3.5 py-3 text-[13px]"><span className="text-link"><Icon name="info" size={18} /></span>
              Ohne „Ölmenge zwischen Min und Max“ in den Fahrzeugdaten wird der Verbrauch nur in Prozentpunkten berechnet.</div>
          )}

          {canEdit && (
            <Card className="p-5">
              <form onSubmit={submit} className="flex flex-col gap-4">
                <CardTitle>Eintrag erfassen</CardTitle>
                <Segmented label="Art" value={d.kind} onChange={(k) => upd({ kind: k })} options={Object.entries(kindLabel) as [Kind, string][]} />
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <Field label="Kilometerstand" htmlFor="o-odo" hint={cur?.meter_value ? `Zuletzt ${fmtNumber(cur.meter_value.canonical / 1000)} km` : undefined}>
                    <input id="o-odo" inputMode="decimal" required={vehicle.odometer_required} className={inputClass + ' tabular'} value={d.odometer} onChange={(e) => upd({ odometer: e.target.value })} />
                  </Field>
                  <Field label={d.dateOnly ? 'Datum' : 'Datum und Uhrzeit'} htmlFor="o-at">
                    <input id="o-at" type={d.dateOnly ? 'date' : 'datetime-local'} required className={inputClass} value={d.dateOnly ? d.at.slice(0, 10) : d.at}
                      onChange={(e) => upd({ at: d.dateOnly ? e.target.value + d.at.slice(10) : e.target.value })} />
                  </Field>
                  {d.kind !== 'oil_change' && <LevelInput id="o-before" label={d.kind === 'top_up' ? 'Ölstand vor dem Nachfüllen' : 'Gemessener Ölstand'}
                    step={d.beforeStep} pct={d.before} onStep={(v) => upd({ beforeStep: v })} onPct={(v) => upd({ before: v })} />}
                  {d.kind === 'top_up' && (
                    <Field label="Nachgefüllte Menge" htmlFor="o-added">
                      <div className="flex gap-2">
                        <input id="o-added" inputMode="decimal" required className={inputClass + ' tabular'} value={d.added} onChange={(e) => upd({ added: e.target.value })} placeholder="0,7" />
                        <select aria-label="Einheit" className={inputClass + ' w-auto'} value={d.addedUnit} onChange={(e) => upd({ addedUnit: e.target.value })}>
                          <option value="l">l</option><option value="ml">ml</option><option value="qt_us">qt (US)</option><option value="qt_imp">qt (UK)</option>
                        </select>
                      </div>
                    </Field>
                  )}
                  {d.kind !== 'check' && <LevelInput id="o-after" label="Ölstand danach (optional)" step={d.afterStep} pct={d.after} onStep={(v) => upd({ afterStep: v })} onPct={(v) => upd({ after: v })} />}
                  {d.kind === 'oil_change' && (
                    <Field label="Einfüllmenge (Liter)" htmlFor="o-fill"><input id="o-fill" inputMode="decimal" className={inputClass + ' tabular'} value={d.fill} onChange={(e) => upd({ fill: e.target.value })} /></Field>
                  )}
                  <Field label="Ölspezifikation" htmlFor="o-spec"><input id="o-spec" className={inputClass} value={d.spec} onChange={(e) => upd({ spec: e.target.value })} placeholder="5W-30, MB 229.52" /></Field>
                  <Field label="Marke / Produkt" htmlFor="o-brand"><input id="o-brand" className={inputClass} value={d.brand} onChange={(e) => upd({ brand: e.target.value })} /></Field>
                </div>
                <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm">
                  {d.kind === 'oil_change' && <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={d.filter} onChange={(e) => upd({ filter: e.target.checked })} />Ölfilter gewechselt</label>}
                  <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={d.dateOnly} onChange={(e) => upd({ dateOnly: e.target.checked })} />Uhrzeit unbekannt</label>
                </div>
                {d.kind === 'oil_change' && <div className="text-xs text-muted">Tipp: Die Wartung „Ölwechsel“ unter Wartung als erledigt markieren, damit die nächste Fälligkeit stimmt.</div>}
                {save.error && <div role="alert" className="text-sm font-semibold text-bad">{save.error}</div>}
                {save.anomalies && <PlausibilityAlert anomalies={save.anomalies} busy={save.busy} onEdit={save.reset} onConfirm={save.confirm} />}
                {!save.anomalies && <Button type="submit" icon="plus" disabled={save.busy} className="self-start">Speichern</Button>}
              </form>
            </Card>
          )}

          <Card className="flex flex-col gap-3 p-5">
            <CardTitle>Messreihen</CardTitle>
            {(series.data?.items ?? []).length === 0 && <div className="text-sm text-muted">Noch keine Einträge.</div>}
            {(series.data?.items ?? []).map((s) => (
              <div key={s.id} className="flex items-center justify-between gap-3 border-t border-line pt-3 first:border-t-0 first:pt-0">
                <div className="flex flex-col gap-0.5">
                  <div className="text-sm font-semibold">{s.started_at ? `Ölwechsel am ${fmtDate(s.started_at, true)}` : 'Vor dem ersten erfassten Ölwechsel'}</div>
                  <div className="text-xs text-muted">{s.usable_measurements} verwertbare Messungen{s.distance_since_start ? ` · ${fmtDisplay(s.distance_since_start, 0)}` : ''}</div>
                </div>
                {s.status === 'ok' && s.consumption_per_1000 ? <span className="tabular text-sm font-semibold">{fmtDisplay(s.consumption_per_1000, 0)}</span> : <Chip>zu wenige Messungen</Chip>}
              </div>
            ))}
          </Card>
        </div>

        <Card className="flex flex-col overflow-hidden">
          <div className="flex items-center justify-between px-5 py-4"><h2 className="m-0 font-display text-base font-semibold">Verlauf</h2><div className="text-[13px] text-muted">Verbrauch zum vorigen Stand</div></div>
          {items.length === 0 && <div className="border-t border-line px-5 py-8 text-sm text-muted">Noch keine Einträge.</div>}
          {items.map((e) => <EntryRow key={e.id} e={e} vid={vehicle.id} canEdit={canEdit} />)}
        </Card>
      </main>
    </>
  )
}

function levelText(l?: Schemas['OilLevelInput'] | null) {
  if (!l) return null
  if (l.step) return steps.find(([k]) => k === l.step)?.[1] ?? l.step
  return l.percent != null ? `${fmtNumber(l.percent)} %` : null
}

function EntryRow({ e, vid, canEdit }: { e: Entry; vid: string; canEdit: boolean }) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  async function remove() {
    if (!confirm('Diesen Eintrag löschen? Der zugehörige Kilometerstand wird ebenfalls entfernt.')) return
    setBusy(true)
    try { await api.del(`/vehicles/${vid}/oil-entries/${e.id}`, `"${e.version}"`) } finally {
      setBusy(false)
      qc.invalidateQueries({ queryKey: oilKeys(vid) })
      qc.invalidateQueries({ queryKey: ['odometer', vid] })
    }
  }
  const p = e.pair
  const parts = [levelText(e.level_before), e.oil_added ? `+${fmtNumber(e.oil_added.value, 2)} ${e.oil_added.unit}` : null,
    e.level_after ? `danach ${levelText(e.level_after)}` : e.level_after_pct_computed != null ? `danach ≈ ${fmtNumber(e.level_after_pct_computed)} %` : null].filter(Boolean)
  return (
    <div className="grid min-h-[64px] grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-2">
      <div className={`flex h-9 w-9 items-center justify-center rounded-[10px] ${e.kind === 'oil_change' ? 'bg-ok-bg text-ok' : 'bg-info-bg text-link'}`}><Icon name={e.kind === 'oil_change' ? 'sync' : 'oil'} /></div>
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="text-[15px] font-semibold">{kindLabel[e.kind as Kind]}{parts.length > 0 && <span className="font-normal text-muted"> · {parts.join(' · ')}</span>}</div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
          <span>{fmtDate(e.occurred_at, e.time_precision === 'date_only')}</span>
          {e.odometer && <span className="tabular">{fmtNumber(e.odometer.value)} {e.odometer.unit}</span>}
          {e.oil_specification && <span>{e.oil_specification}</span>}
        </div>
      </div>
      <div className="flex items-center gap-2">
        {p?.status === 'computed' && <span className={`tabular text-sm font-semibold ${p.negative ? 'text-warn' : 'text-ok'}`} title={p.negative ? 'Stand gestiegen: Messung ungenau oder Kraftstoffeintrag ins Öl prüfen' : undefined}>{fmtDisplay(p.consumption_per_1000, 0)}</span>}
        {p?.status === 'not_computable' && <Chip tone="warn">{pairReason[p.reason ?? ''] ?? 'nicht berechenbar'}</Chip>}
        {canEdit && <button type="button" disabled={busy} onClick={remove} aria-label="Löschen" className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="x" size={18} /></button>}
      </div>
    </div>
  )
}
