import { useRef, useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Anomaly } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { PlausibilityAlert } from '../components/Plausibility'
import { Dialog } from '../components/Dialog'
import { Button, Card, CardTitle, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { browserTimeZone, fmtDate, fmtNumber, localInputValue } from '../lib/format'
import { odoKeys, sourceInfo, useCurrent, useMonthly, useReadings, type Reading } from '../lib/odometer'
import { useApp } from '../lib/state'

interface Draft { value: string; at: string; dateOnly: boolean; note: string }

function toBody(d: Draft) {
  const at = d.dateOnly ? new Date(d.at.slice(0, 10) + 'T12:00') : new Date(d.at)
  return {
    occurred_at: at.toISOString(), time_zone: browserTimeZone(), time_precision: d.dateOnly ? 'date_only' : 'exact',
    value: { value: Number(d.value.replace(',', '.')), unit: 'km' }, note: d.note || undefined,
  }
}

export function OdometerPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const qc = useQueryClient()
  const current = useCurrent(vid)
  const readings = useReadings(vid, true)
  const monthly = useMonthly(vid)
  const [draft, setDraft] = useState<Draft>({ value: '', at: localInputValue(), dateOnly: false, note: '' })
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const [correcting, setCorrecting] = useState<Reading | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'

  const create = useMutation({
    mutationFn: (extra: object) => api.post(`/vehicles/${vid}/odometer/readings`, { ...toBody(draft), ...extra }),
    onSuccess: () => {
      setAnomalies(null)
      setDraft({ value: '', at: localInputValue(), dateOnly: false, note: '' })
      qc.invalidateQueries({ queryKey: odoKeys(vid!) })
    },
    onError: (e) => {
      if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
      else setError(e instanceof ProblemError ? (e.problem.errors?.[0]?.message || e.problem.detail || e.problem.title) : 'Speichern fehlgeschlagen.')
    },
  })

  if (!vehicle) return <><Header title="Kilometerstand" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>

  const items = readings.data?.items ?? []
  const valid = items.filter((r) => r.status !== 'superseded')
  const maxKm = Math.max(1, ...(monthly.data ?? []).map((m) => m.km ?? 0))
  const cur = current.data

  function submit(e: FormEvent) {
    e.preventDefault()
    setError(undefined)
    setAnomalies(null)
    create.mutate({})
  }

  return (
    <>
      <Header title="Kilometerstand" sub={vehicle.display_name} />
      <main className="grid min-h-0 flex-grow grid-cols-1 items-start gap-5 overflow-y-auto px-8 py-6 xl:grid-cols-2">
        <div className="flex flex-col gap-5">
          <section className="flex flex-col gap-1.5 rounded-[16px] bg-hero p-5">
            <div className="flex items-center gap-2 text-sm text-slate-300"><span className="text-teal-soft"><Icon name="gauge" /></span>Aktueller Stand</div>
            <div className="tabular font-display text-[40px] font-semibold text-paper">
              {cur && cur.kind !== 'unknown' && cur.meter_value ? `${fmtNumber(cur.meter_value.canonical / 1000)} km` : '–'}
            </div>
            <div className="text-[13px] text-slate-300">
              {cur && cur.kind !== 'unknown' ? `Zuletzt am ${fmtDate(cur.at)}` : 'Noch kein Stand erfasst'}
            </div>
          </section>

          {canEdit && (
            <Card className="p-5">
              <form onSubmit={submit} className="flex flex-col gap-4">
                <CardTitle>Stand erfassen</CardTitle>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <Field label="Kilometerstand" htmlFor="odo-value" hint={cur?.meter_value ? `Letzter Stand: ${fmtNumber(cur.meter_value.canonical / 1000)} km` : undefined}>
                    <div className="relative">
                      <input id="odo-value" ref={inputRef} inputMode="decimal" required className={inputClass + ' tabular pr-12'} value={draft.value}
                        onChange={(e) => setDraft({ ...draft, value: e.target.value })} placeholder="143520" />
                      <span className="absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted">km</span>
                    </div>
                  </Field>
                  <Field label={draft.dateOnly ? 'Datum' : 'Datum und Uhrzeit'} htmlFor="odo-at">
                    <input id="odo-at" type={draft.dateOnly ? 'date' : 'datetime-local'} required className={inputClass}
                      value={draft.dateOnly ? draft.at.slice(0, 10) : draft.at}
                      onChange={(e) => setDraft({ ...draft, at: draft.dateOnly ? e.target.value + draft.at.slice(10) : e.target.value })} />
                  </Field>
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" className="h-4 w-4 accent-teal" checked={draft.dateOnly} onChange={(e) => setDraft({ ...draft, dateOnly: e.target.checked })} />
                  Uhrzeit unbekannt (z. B. Beleg nur mit Datum)
                </label>
                <Field label="Notiz" htmlFor="odo-note"><input id="odo-note" className={inputClass} value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} /></Field>
                {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
                {anomalies && (
                  <PlausibilityAlert anomalies={anomalies} busy={create.isPending}
                    onEdit={() => { setAnomalies(null); inputRef.current?.focus() }}
                    onConfirm={(codes, reason) => create.mutate({ confirm_anomalies: codes, anomaly_reason: reason })} />
                )}
                {!anomalies && <Button type="submit" icon="plus" disabled={create.isPending} className="self-start">Stand speichern</Button>}
              </form>
            </Card>
          )}

          <Card className="flex flex-col gap-3.5 p-5">
            <CardTitle aside={<span className="text-[13px] text-muted">letzte 6 Monate</span>}>Gefahrene km pro Monat</CardTitle>
            <div className="flex min-h-[180px] items-end gap-4" role="list" aria-label="Gefahrene Kilometer je Monat">
              {(monthly.data ?? []).map((m, i, arr) => (
                <div key={m.key} role="listitem" className="flex flex-grow flex-col items-center gap-1.5">
                  <div className="tabular text-xs text-muted">{m.km === null ? '–' : fmtNumber(m.km)}</div>
                  <div className={`w-full rounded-[8px_8px_4px_4px] ${i === arr.length - 1 ? 'bg-teal' : 'bg-bar'}`}
                    style={{ height: `${m.km ? Math.max(4, Math.round((m.km / maxKm) * 150)) : 4}px` }} />
                  <div className="text-xs font-semibold">{m.label}</div>
                </div>
              ))}
            </div>
          </Card>
        </div>

        <Card className="flex flex-col overflow-hidden">
          <div className="flex items-center justify-between px-5 py-4"><h2 className="m-0 font-display text-base font-semibold">Verlauf</h2><div className="text-[13px] text-muted">Herkunft jedes Stands</div></div>
          {items.length === 0 && <div className="border-t border-line px-5 py-8 text-sm text-muted">Noch keine Messpunkte.</div>}
          {items.map((r) => {
            const src = sourceInfo[r.source] ?? sourceInfo.manual
            const idx = valid.findIndex((v) => v.id === r.id)
            const prev = idx >= 0 ? valid[idx + 1] : undefined
            const delta = prev ? (r.total.canonical - prev.total.canonical) / 1000 : null
            const superseded = r.status === 'superseded'
            return (
              <div key={r.id} className={`grid min-h-[64px] grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-2 ${superseded ? 'opacity-60' : ''}`}>
                <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name={src.icon} /></div>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <div className={`tabular text-[15px] font-semibold ${superseded ? 'line-through' : ''}`}>{fmtNumber(r.meter_value.canonical / 1000)} km</div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                    <span>{src.label} · {fmtDate(r.occurred_at, r.time_precision === 'date_only')}</span>
                    {r.status === 'confirmed_anomaly' && <Chip tone="warn" icon="alert">bestätigte Abweichung</Chip>}
                    {superseded && <Chip>ersetzt</Chip>}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {delta !== null && !superseded && <span className={`tabular text-sm font-semibold ${delta < 0 ? 'text-warn' : 'text-ok'}`}>{delta >= 0 ? '+' : ''}{fmtNumber(delta)} km</span>}
                  {canEdit && !superseded && (
                    <button type="button" onClick={() => setCorrecting(r)} aria-label="Korrigieren" className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="edit" size={18} /></button>
                  )}
                </div>
              </div>
            )
          })}
        </Card>
      </main>
      {correcting && <CorrectDialog vid={vehicle.id} reading={correcting} onClose={() => setCorrecting(null)} />}
    </>
  )
}

function CorrectDialog({ vid, reading, onClose }: { vid: string; reading: Reading; onClose: () => void }) {
  const qc = useQueryClient()
  const [value, setValue] = useState(String(reading.meter_value.canonical / 1000))
  const [reason, setReason] = useState('')
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const base = `/vehicles/${vid}/odometer/readings/${reading.id}`
  const etag = `"${reading.version}"`
  const done = () => { qc.invalidateQueries({ queryKey: odoKeys(vid) }); onClose() }
  const onErr = (e: unknown) => {
    if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
    else setError(e instanceof ProblemError ? (e.problem.errors?.[0]?.message || e.problem.detail || e.problem.title) : 'Fehlgeschlagen.')
  }
  const correct = useMutation({
    mutationFn: (extra: object) => api.post(base + '/corrections', { value: { value: Number(value.replace(',', '.')), unit: 'km' }, reason, ...extra }, { 'If-Match': etag }),
    onSuccess: done, onError: onErr,
  })
  const remove = useMutation({ mutationFn: () => api.del(base, etag), onSuccess: done, onError: onErr })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title="Stand korrigieren">
      <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); setAnomalies(null); correct.mutate({}) }}>
        <p className="m-0 text-sm leading-relaxed text-muted">Korrekturen überschreiben nichts: Der alte Wert bleibt als „ersetzt“ in der Historie sichtbar.</p>
        <Field label="Richtiger Kilometerstand" htmlFor="corr-value"><input id="corr-value" inputMode="decimal" required className={inputClass + ' tabular'} value={value} onChange={(e) => setValue(e.target.value)} /></Field>
        <Field label="Begründung" htmlFor="corr-reason" error={error}><input id="corr-reason" required className={inputClass} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="z. B. Tippfehler" /></Field>
        {anomalies && <PlausibilityAlert anomalies={anomalies} busy={correct.isPending} onEdit={() => setAnomalies(null)}
          onConfirm={(codes, r) => correct.mutate({ confirm_anomalies: codes, anomaly_reason: r })} />}
        <div className="flex flex-wrap justify-between gap-3">
          {reading.source === 'manual' ? <Button type="button" variant="outline" onClick={() => remove.mutate()} disabled={remove.isPending}>Messpunkt löschen</Button> : <span />}
          {!anomalies && <Button type="submit" disabled={correct.isPending || !reason.trim()}>Korrektur speichern</Button>}
        </div>
      </form>
    </Dialog>
  )
}
