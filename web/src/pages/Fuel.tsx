import { useState, type FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Card, CardTitle, Chip, EmptyState, Field, Kpi, inputClass } from '../components/ui'
import { browserTimeZone, carrierLabel, fmtDate, fmtDisplay, fmtMoney, fmtNumber, localInputValue, num, toIso } from '../lib/format'
import { usePlausibleMutation } from '../lib/mutation'
import { useCurrent } from '../lib/odometer'
import { useApp } from '../lib/state'

type Fill = Schemas['FuelFill']
type Summary = Schemas['ConsumptionSummary']

const reasonLabel: Record<string, string> = {
  previous_missed: 'Tankvorgang ausgelassen', non_positive_distance: 'Distanz ≤ 0', no_anchor: 'erste Vollbetankung', no_odometer: 'ohne Kilometerstand',
}

export const fuelKeys = (vid: string) => ['fuel', vid]

interface Draft {
  at: string; dateOnly: boolean; carrier: string; quantity: string; full: boolean; odometer: string
  priceMode: 'total' | 'unit'; amount: string; missed: boolean; station: string; socStart: string; socEnd: string; chargeType: string; note: string
}

function emptyDraft(carrier: string): Draft {
  return { at: localInputValue(), dateOnly: false, carrier, quantity: '', full: true, odometer: '', priceMode: 'total', amount: '', missed: false,
    station: '', socStart: '', socEnd: '', chargeType: '', note: '' }
}

function bodyOf(d: Draft, currency: string) {
  const electric = d.carrier === 'electricity'
  const b: Record<string, unknown> = {
    occurred_at: toIso(d.at, d.dateOnly), time_zone: browserTimeZone(), time_precision: d.dateOnly ? 'date_only' : 'exact',
    energy_carrier: d.carrier, quantity: { value: num(d.quantity), unit: electric ? 'kWh' : 'l' }, fill_level: d.full ? 'full' : 'partial',
    previous_missed: d.missed, note: d.note,
  }
  if (d.odometer) b.odometer = { value: num(d.odometer), unit: 'km' }
  if (d.amount) {
    if (d.priceMode === 'total') b.cost = { amount_minor: Math.round(num(d.amount) * 100), currency }
    else b.price_per_unit = { value: num(d.amount), currency, per_unit: electric ? 'kWh' : 'l' }
  }
  if (d.station) b.station = d.station
  if (electric) {
    if (d.socStart) b.soc_start_pct = num(d.socStart)
    if (d.socEnd) b.soc_end_pct = num(d.socEnd)
    if (d.chargeType) b.charge_type = d.chargeType
  }
  return b
}

export function FuelPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const qc = useQueryClient()
  const carriers = vehicle?.energy_carriers?.length ? vehicle.energy_carriers : ['petrol']
  const [carrier, setCarrier] = useState<string>()
  const active = carrier && carriers.includes(carrier as never) ? carrier : carriers[0]
  const [draft, setDraft] = useState<Draft>(() => emptyDraft(active))
  const cur = useCurrent(vid).data
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'
  const currency = vehicle?.default_currency ?? 'EUR'

  const fills = useQuery({
    enabled: !!vid, queryKey: [...fuelKeys(vid!), 'fills', active],
    queryFn: () => api.get<{ items: Fill[] }>(`/vehicles/${vid}/fuel-fills?limit=100&energy_carrier=${active}`),
  })
  const summary = useQuery({
    enabled: !!vid, queryKey: [...fuelKeys(vid!), 'summary', active],
    queryFn: () => api.get<Summary>(`/vehicles/${vid}/fuel/consumption?energy_carrier=${active}`),
  })
  const save = usePlausibleMutation(
    (extra) => api.post(`/vehicles/${vid}/fuel-fills`, { ...bodyOf({ ...draft, carrier: active }, currency), ...extra }),
    () => { setDraft(emptyDraft(active)); qc.invalidateQueries({ queryKey: fuelKeys(vid!) }); qc.invalidateQueries({ queryKey: ['odometer', vid] }) },
  )

  if (!vehicle) return <><Header title="Kraftstoff" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>

  const electric = active === 'electricity'
  const s = summary.data
  const series = electric ? s?.grid : s?.consumption
  const monthly = series?.monthly ?? []
  const max = Math.max(1, ...monthly.map((m) => m.value?.value ?? 0))
  const items = fills.data?.items ?? []
  const set = (k: keyof Draft) => (e: { target: { value: string } }) => setDraft({ ...draft, [k]: e.target.value })

  function submit(e: FormEvent) {
    e.preventDefault()
    save.submit()
  }

  return (
    <>
      <Header title="Kraftstoff" sub={`${vehicle.display_name} · ${electric ? 'Laden' : 'Tanken'}`}
        actions={carriers.length > 1 ? (
          <div role="tablist" className="flex gap-1 rounded-[12px] bg-soft p-1">
            {carriers.map((c) => (
              <button key={c} type="button" role="tab" aria-selected={c === active} onClick={() => { setCarrier(c); setDraft(emptyDraft(c)) }}
                className={`h-8 rounded-[9px] px-3 text-sm font-semibold ${c === active ? 'bg-card text-text shadow-sm' : 'text-muted'}`}>{carrierLabel[c]}</button>
            ))}
          </div>
        ) : undefined} />
      <main className="grid min-h-0 flex-grow grid-cols-1 items-start gap-5 overflow-y-auto px-8 py-6 xl:grid-cols-2">
        <div className="flex flex-col gap-5">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Kpi icon="fuel" label={electric ? 'Ø Netzbezug' : 'Ø Verbrauch'} value={fmtDisplay(series?.average)}
              note={series ? `${series.computed_intervals} Intervalle${series.not_computable_intervals ? `, ${series.not_computable_intervals} nicht berechenbar` : ''}` : 'Noch keine Daten'} />
            {electric
              ? <Kpi icon="trend" label="Ø Batterie" value={fmtDisplay(s?.battery?.average)} note={s?.battery_unavailable_reason === 'capacity_unknown' ? 'Nutzbare Kapazität am Fahrzeug fehlt' : s?.battery_unavailable_reason === 'soc_missing' ? 'Ladezustände fehlen' : 'aus Ladezuständen'} />
              : <Kpi icon="gauge" label="Stand" value={cur?.meter_value ? `${fmtNumber(cur.meter_value.canonical / 1000)} km` : '–'} note="aktueller Kilometerstand" />}
            <Kpi icon="euro" label="Ø Preis" value={fmtDisplay(s?.average_unit_price, 3)} note="aus Vorgängen mit Betrag" />
          </div>

          {canEdit && (
            <Card className="p-5">
              <form onSubmit={submit} className="flex flex-col gap-4">
                <CardTitle>{electric ? 'Ladevorgang erfassen' : 'Tankvorgang erfassen'}</CardTitle>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <Field label={electric ? 'Energie (kWh)' : 'Menge (Liter)'} htmlFor="f-qty">
                    <input id="f-qty" inputMode="decimal" required className={inputClass + ' tabular'} value={draft.quantity} onChange={set('quantity')} placeholder={electric ? '42,5' : '55,20'} />
                  </Field>
                  <Field label="Kilometerstand" htmlFor="f-odo" hint={cur?.meter_value ? `Zuletzt ${fmtNumber(cur.meter_value.canonical / 1000)} km` : undefined}>
                    <input id="f-odo" inputMode="decimal" required={vehicle.odometer_required} className={inputClass + ' tabular'} value={draft.odometer} onChange={set('odometer')} />
                  </Field>
                  <Field label={draft.dateOnly ? 'Datum' : 'Datum und Uhrzeit'} htmlFor="f-at">
                    <input id="f-at" type={draft.dateOnly ? 'date' : 'datetime-local'} required className={inputClass}
                      value={draft.dateOnly ? draft.at.slice(0, 10) : draft.at}
                      onChange={(e) => setDraft({ ...draft, at: draft.dateOnly ? e.target.value + draft.at.slice(10) : e.target.value })} />
                  </Field>
                  <Field label={draft.priceMode === 'total' ? `Betrag (${currency})` : `Preis pro ${electric ? 'kWh' : 'Liter'}`} htmlFor="f-amount"
                    hint={draft.priceMode === 'unit' && draft.amount && draft.quantity ? `Vorschau: ${fmtMoney(Math.round(num(draft.amount) * num(draft.quantity) * 100), currency)} (der Server rechnet)` : undefined}>
                    <div className="flex gap-2">
                      <input id="f-amount" inputMode="decimal" className={inputClass + ' tabular'} value={draft.amount} onChange={set('amount')} />
                      <select aria-label="Preisangabe" className={inputClass + ' w-auto'} value={draft.priceMode} onChange={(e) => setDraft({ ...draft, priceMode: e.target.value as Draft['priceMode'] })}>
                        <option value="total">gesamt</option><option value="unit">je Einheit</option>
                      </select>
                    </div>
                  </Field>
                  {electric && <>
                    <Field label="Ladezustand Beginn / Ende (%)" htmlFor="f-soc">
                      <div className="flex gap-2">
                        <input id="f-soc" inputMode="decimal" className={inputClass} value={draft.socStart} onChange={set('socStart')} placeholder="20" />
                        <input aria-label="Ladezustand Ende" inputMode="decimal" className={inputClass} value={draft.socEnd} onChange={set('socEnd')} placeholder="80" />
                      </div>
                    </Field>
                    <Field label="Ladeart" htmlFor="f-ct">
                      <select id="f-ct" className={inputClass} value={draft.chargeType} onChange={set('chargeType')}>
                        <option value="">unbekannt</option><option value="ac">AC</option><option value="dc">DC</option>
                      </select>
                    </Field>
                  </>}
                  <Field label={electric ? 'Ladepunkt' : 'Tankstelle'} htmlFor="f-station"><input id="f-station" className={inputClass} value={draft.station} onChange={set('station')} /></Field>
                  <Field label="Notiz" htmlFor="f-note"><input id="f-note" className={inputClass} value={draft.note} onChange={set('note')} /></Field>
                </div>
                <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm">
                  <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={draft.full} onChange={(e) => setDraft({ ...draft, full: e.target.checked })} />{electric ? 'bis zum üblichen Ziel geladen' : 'vollgetankt'}</label>
                  <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={draft.missed} onChange={(e) => setDraft({ ...draft, missed: e.target.checked })} />vorigen Vorgang nicht erfasst</label>
                  <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={draft.dateOnly} onChange={(e) => setDraft({ ...draft, dateOnly: e.target.checked })} />Uhrzeit unbekannt</label>
                </div>
                {save.error && <div role="alert" className="text-sm font-semibold text-bad">{save.error}</div>}
                {save.anomalies && <PlausibilityAlert anomalies={save.anomalies} busy={save.busy} onEdit={save.reset} onConfirm={save.confirm} />}
                {!save.anomalies && <Button type="submit" icon="plus" disabled={save.busy} className="self-start">Speichern</Button>}
              </form>
            </Card>
          )}

          <Card className="flex flex-col gap-3.5 p-5">
            <CardTitle aside={<span className="text-[13px] text-muted">gewichtet, je Abschlussmonat</span>}>Verbrauch pro Monat</CardTitle>
            {monthly.length === 0 && <div className="text-sm text-muted">Für Monatswerte braucht es zwei Vollbetankungen mit Kilometerstand.</div>}
            <div className="flex min-h-[160px] items-end gap-3" role="list" aria-label="Verbrauch je Monat">
              {monthly.slice(-12).map((m) => (
                <div key={m.month} role="listitem" className="flex flex-grow flex-col items-center gap-1.5">
                  <div className="tabular text-xs text-muted">{m.value ? fmtNumber(m.value.value, 1) : '–'}</div>
                  <div className="w-full rounded-[8px_8px_4px_4px] bg-teal" style={{ height: `${m.value ? Math.max(4, Math.round((m.value.value / max) * 130)) : 4}px` }} />
                  <div className="text-xs font-semibold">{m.month.slice(5)}/{m.month.slice(2, 4)}</div>
                </div>
              ))}
            </div>
          </Card>
        </div>

        <Card className="flex flex-col overflow-hidden">
          <div className="flex items-center justify-between px-5 py-4"><h2 className="m-0 font-display text-base font-semibold">Verlauf</h2><div className="text-[13px] text-muted">Verbrauch je Intervall</div></div>
          {items.length === 0 && <div className="border-t border-line px-5 py-8 text-sm text-muted">Noch keine Vorgänge.</div>}
          {items.map((f) => <FillRow key={f.id} f={f} vid={vehicle.id} canEdit={canEdit} />)}
        </Card>
      </main>
    </>
  )
}


function FillRow({ f, vid, canEdit }: { f: Fill; vid: string; canEdit: boolean }) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  const iv = f.interval
  const electric = f.energy_carrier === 'electricity'
  async function remove() {
    if (!confirm('Diesen Vorgang löschen? Der zugehörige Kilometerstand wird ebenfalls entfernt.')) return
    setBusy(true)
    try { await api.del(`/vehicles/${vid}/fuel-fills/${f.id}`, `"${f.version}"`) } finally {
      setBusy(false)
      qc.invalidateQueries({ queryKey: fuelKeys(vid) })
      qc.invalidateQueries({ queryKey: ['odometer', vid] })
    }
  }
  return (
    <div className="grid min-h-[64px] grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-2">
      <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name="fuel" /></div>
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="tabular text-[15px] font-semibold">
          {fmtNumber(f.quantity.value, 2)} {electric ? 'kWh' : 'l'}
          {f.cost && <span className="font-normal text-muted"> · {fmtMoney(f.cost.amount_minor, f.cost.currency)}</span>}
          {f.unit_price && <span className="font-normal text-muted"> · {fmtDisplay(f.unit_price, 3)}</span>}
        </div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
          <span>{fmtDate(f.occurred_at, f.time_precision === 'date_only')}</span>
          {f.odometer && <span className="tabular">{fmtNumber(f.odometer.value)} {f.odometer.unit}</span>}
          {f.fill_level === 'partial' && <Chip>Teil</Chip>}
          {f.previous_missed && <Chip tone="warn">Lücke</Chip>}
          {f.station && <span className="truncate">{f.station}</span>}
        </div>
      </div>
      <div className="flex items-center gap-2">
        {iv?.status === 'computed' && <span className="tabular text-sm font-semibold text-ok">{fmtDisplay(iv.consumption)}</span>}
        {iv?.status === 'not_computable' && <Chip tone="warn" icon="alert">{reasonLabel[iv.reason ?? ''] ?? 'nicht berechenbar'}</Chip>}
        {iv?.status === 'anchor' && <span className="text-xs text-muted">Startpunkt</span>}
        {canEdit && <button type="button" disabled={busy} onClick={remove} aria-label="Löschen" className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="x" size={18} /></button>}
      </div>
    </div>
  )
}
