import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Anomaly, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Card, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { browserTimeZone, fmtNumber } from '../lib/format'
import { errorText, etagOf, fmtDay, fmtMoney, noonISO, parseMoney, parseNumber, today, toMoneyInput } from '../lib/money'
import { useApp } from '../lib/state'
import { useMaintenance } from './Maintenance'

type Entry = Schemas['ServiceEntry'] & { version: number; totals?: Schemas['ServiceTotals'] | null }

const kindLabel: Record<string, string> = { maintenance: 'Wartung', inspection: 'Inspektion', repair: 'Reparatur', upgrade: 'Nachrüstung' }
const costKindLabel: Record<string, string> = { parts: 'Teile', labor: 'Arbeit', other: 'Sonstiges' }

export function useServiceEntries(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: ['service', vid, 'list'], queryFn: () => api.get<{ items: Entry[] }>(`/vehicles/${vid}/service-entries?limit=100`) })
}

export function ServicePage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const [kind, setKind] = useState('')
  const [q, setQ] = useState('')
  const [editing, setEditing] = useState<Entry | 'new' | null>(null)
  const year = new Date().getFullYear()
  const list = useQuery({
    enabled: !!vid, queryKey: ['service', vid, 'list', kind, q],
    queryFn: () => api.get<{ items: Entry[] }>(`/vehicles/${vid}/service-entries?limit=100${kind ? '&kind=' + kind : ''}${q ? '&q=' + encodeURIComponent(q) : ''}`),
  })
  const summary = useQuery({ enabled: !!vid, queryKey: ['service', vid, 'summary', year], queryFn: () => api.get<Schemas['ServiceSummary']>(`/vehicles/${vid}/service/summary?from=${year}-01-01&to=${year}-12-31`) })
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'

  if (!vehicle) return <><Header title="Servicehistorie" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>
  const items = list.data?.items ?? []
  const cur = summary.data?.currencies?.[0]

  return (
    <>
      <Header title="Servicehistorie" sub={`${vehicle.display_name} · Werkstatt, Reparaturen, Nachrüstungen`}
        actions={canEdit && <Button icon="plus" onClick={() => setEditing('new')}>Eintrag erfassen</Button>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <Kpi label={`Service ${year}`} value={cur ? fmtMoney(cur.total_minor, cur.currency) : '–'} note={`${summary.data?.entries ?? 0} Einträge`} />
          {(['parts', 'labor', 'other'] as const).map((k) => (
            <Kpi key={k} label={costKindLabel[k]} value={cur ? fmtMoney(cur.by_cost_kind?.[k] ?? 0, cur.currency) : '–'}
              note={k === 'other' && summary.data?.entries_cost_unknown ? `${summary.data.entries_cost_unknown} ohne Kosten` : ' '} />
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative min-w-[240px] flex-grow">
            <span className="absolute top-1/2 left-3.5 -translate-y-1/2 text-muted"><Icon name="search" size={18} /></span>
            <input aria-label="Suchen" className={inputClass + ' h-11 pl-10'} placeholder="Titel, Werkstatt, Teil, Rechnungsnummer …" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <select aria-label="Art" className={inputClass + ' h-11 w-auto'} value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="">Alle Arten</option>
            {Object.entries(kindLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}
          </select>
        </div>
        {items.length === 0 && !list.isPending && (
          <EmptyState icon="receipt" title="Noch keine Serviceeinträge" text="Erfasse Werkstattbesuche mit Teilen, Arbeitslohn und Kilometerstand. Erledigte Wartungen hakst du dabei direkt ab."
            action={canEdit && !q && !kind && <Button icon="plus" onClick={() => setEditing('new')}>Eintrag erfassen</Button>} />
        )}
        {items.length > 0 && (
          <Card className="overflow-hidden">
            {items.map((e, i) => (
              <button key={e.id} type="button" disabled={!canEdit} onClick={() => setEditing(e)}
                className={`grid w-full grid-cols-[40px_minmax(0,1fr)_auto] items-center gap-3 px-5 py-3 text-left text-text hover:bg-soft disabled:hover:bg-transparent ${i ? 'border-t border-line' : ''}`}>
                <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name={e.kind === 'repair' ? 'alert' : 'wrench'} /></div>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <div className="flex flex-wrap items-center gap-2"><span className="truncate text-[15px] font-semibold">{e.title}</span><Chip>{kindLabel[e.kind]}</Chip>
                    {(e.completes_maintenance_item_ids?.length ?? 0) > 0 && <Chip tone="ok" icon="check">{e.completes_maintenance_item_ids!.length} Wartung{e.completes_maintenance_item_ids!.length > 1 ? 'en' : ''}</Chip>}</div>
                  <div className="text-xs text-muted">{[fmtDay(e.occurred_at), e.odometer ? `${fmtNumber(e.odometer.value)} ${e.odometer.unit}` : '', e.provider_name].filter(Boolean).join(' · ')}</div>
                </div>
                <div className="tabular text-right text-[15px] font-semibold">{e.totals ? fmtMoney(e.totals.total.amount_minor, e.currency) : <span className="text-sm font-normal text-muted">Kosten unbekannt</span>}</div>
              </button>
            ))}
          </Card>
        )}
      </main>
      {editing && <EntryDialog vid={vehicle.id} currency={vehicle.default_currency ?? 'EUR'} entry={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
    </>
  )
}

function Kpi({ label, value, note }: { label: string; value: string; note: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-[16px] border border-line bg-card p-4">
      <div className="text-[13px] font-medium text-muted">{label}</div>
      <div className="tabular font-display text-[24px] font-semibold">{value}</div>
      <div className="text-xs text-muted">{note}</div>
    </div>
  )
}

interface Line { kind: string; label: string; amount: string }

function EntryDialog({ vid, currency, entry, onClose }: { vid: string; currency: string; entry: Entry | null; onClose: () => void }) {
  const qc = useQueryClient()
  const maint = useMaintenance(vid).data?.items ?? []
  const cur = entry?.currency ?? currency
  const [f, setF] = useState({
    date: entry ? entry.occurred_at.slice(0, 10) : today(), kind: entry?.kind ?? 'maintenance', title: entry?.title ?? '', provider: entry?.provider_name ?? '',
    invoice: entry?.invoice_number ?? '', km: entry?.odometer ? String(entry.odometer.value) : '', note: entry?.note ?? '', costUnknown: entry?.cost_unknown ?? false,
  })
  const [lines, setLines] = useState<Line[]>(entry?.cost_items?.length ? entry.cost_items.map((c) => ({ kind: c.kind, label: c.label ?? '', amount: toMoneyInput(c.amount_minor, cur) }))
    : [{ kind: 'parts', label: '', amount: '' }, { kind: 'labor', label: '', amount: '' }])
  const [completes, setCompletes] = useState<string[]>(entry?.completes_maintenance_item_ids ?? [])
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const total = lines.reduce((s, l) => s + (parseMoney(l.amount, cur) ?? 0), 0)
  const done = () => { ['service', 'maintenance', 'costs', 'odometer'].forEach((k) => qc.invalidateQueries({ queryKey: [k, vid] })); onClose() }
  const onError = (e: unknown) => {
    if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
    else setError(errorText(e))
  }
  const save = useMutation({
    mutationFn: (extra: object) => {
      const km = parseNumber(f.km)
      const items = f.costUnknown ? [] : lines.filter((l) => l.amount.trim() !== '').map((l) => ({ kind: l.kind, label: l.label || null, amount_minor: parseMoney(l.amount, cur) ?? 0 }))
      const body = {
        occurred_at: noonISO(f.date), time_zone: browserTimeZone(), time_precision: 'date_only', kind: f.kind, title: f.title, currency: cur,
        provider_name: f.provider || null, invoice_number: f.invoice || null, odometer: km === null ? null : { value: km, unit: 'km' },
        cost_unknown: f.costUnknown, cost_items: items, completes_maintenance_item_ids: completes, note: f.note, ...extra,
      }
      return entry ? api.patch(`/vehicles/${vid}/service-entries/${entry.id}`, body, etagOf(entry)) : api.post(`/vehicles/${vid}/service-entries`, body)
    },
    onSuccess: done, onError,
  })
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vid}/service-entries/${entry!.id}`, etagOf(entry!)), onSuccess: done, onError })
  const setLine = (i: number, patch: Partial<Line>) => setLines(lines.map((l, j) => (j === i ? { ...l, ...patch } : l)))

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={entry ? 'Serviceeintrag bearbeiten' : 'Serviceeintrag erfassen'}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); setAnomalies(null); save.mutate({}) }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Titel" htmlFor="s-title"><input id="s-title" required className={inputClass} value={f.title} onChange={set('title')} placeholder="Inspektion 60.000 km" /></Field>
          <Field label="Art" htmlFor="s-kind">
            <select id="s-kind" className={inputClass} value={f.kind} onChange={set('kind')}>{Object.entries(kindLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select>
          </Field>
          <Field label="Datum" htmlFor="s-date"><input id="s-date" type="date" required max={today()} className={inputClass} value={f.date} onChange={set('date')} /></Field>
          <Field label="Kilometerstand" htmlFor="s-km"><input id="s-km" inputMode="decimal" className={inputClass + ' tabular'} value={f.km} onChange={set('km')} /></Field>
          <Field label="Werkstatt" htmlFor="s-prov"><input id="s-prov" className={inputClass} value={f.provider} onChange={set('provider')} placeholder="oder „Eigenleistung“" /></Field>
          <Field label="Rechnungsnummer" htmlFor="s-inv"><input id="s-inv" className={inputClass} value={f.invoice} onChange={set('invoice')} /></Field>
        </div>

        <fieldset className="m-0 flex flex-col gap-2 border-0 p-0">
          <legend className="mb-1.5 text-sm font-semibold">Kosten ({cur})</legend>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" className="h-4 w-4 accent-teal" checked={f.costUnknown} onChange={(e) => setF({ ...f, costUnknown: e.target.checked })} />Kosten unbekannt
          </label>
          {!f.costUnknown && (
            <>
              {lines.map((l, i) => (
                <div key={i} className="grid grid-cols-[110px_minmax(0,1fr)_110px_36px] gap-2">
                  <select aria-label="Kostenart" className={inputClass + ' h-10 px-2 text-sm'} value={l.kind} onChange={(e) => setLine(i, { kind: e.target.value })}>
                    {Object.entries(costKindLabel).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
                  </select>
                  <input aria-label="Bezeichnung" className={inputClass + ' h-10 text-sm'} value={l.label} onChange={(e) => setLine(i, { label: e.target.value })} placeholder="z. B. Ölfilter, Entsorgung" />
                  <input aria-label="Betrag" inputMode="decimal" className={inputClass + ' tabular h-10 text-right text-sm'} value={l.amount} onChange={(e) => setLine(i, { amount: e.target.value })} placeholder="0,00" />
                  <button type="button" aria-label="Position entfernen" onClick={() => setLines(lines.filter((_, j) => j !== i))} className="flex h-10 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="x" size={16} /></button>
                </div>
              ))}
              <div className="flex items-center justify-between">
                <Button type="button" variant="ghost" icon="plus" onClick={() => setLines([...lines, { kind: 'other', label: '', amount: '' }])}>Position</Button>
                <div className="tabular text-sm font-semibold">Summe {fmtMoney(total, cur)}</div>
              </div>
            </>
          )}
        </fieldset>

        {maint.length > 0 && (
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mb-1.5 text-sm font-semibold">Erledigte Wartungen</legend>
            {maint.filter((m) => m.active !== false || completes.includes(m.id)).map((m) => (
              <label key={m.id} className="flex items-center gap-2 text-sm">
                <input type="checkbox" className="h-4 w-4 accent-teal" checked={completes.includes(m.id)}
                  onChange={() => setCompletes(completes.includes(m.id) ? completes.filter((x) => x !== m.id) : [...completes, m.id])} />{m.title}
              </label>
            ))}
          </fieldset>
        )}
        <Field label="Notiz" htmlFor="s-note"><input id="s-note" className={inputClass} value={f.note} onChange={set('note')} /></Field>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        {anomalies && <PlausibilityAlert anomalies={anomalies} busy={save.isPending} onEdit={() => setAnomalies(null)}
          onConfirm={(codes, r) => save.mutate({ confirm_anomalies: codes, anomaly_reason: r })} />}
        {!anomalies && (
          <div className="flex flex-wrap justify-between gap-3">
            {entry ? <Button type="button" variant="outline" disabled={remove.isPending} onClick={() => remove.mutate()}>Löschen</Button> : <span />}
            <Button type="submit" disabled={save.isPending || !f.title.trim()}>Speichern</Button>
          </div>
        )}
      </form>
    </Dialog>
  )
}

