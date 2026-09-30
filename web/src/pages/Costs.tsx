import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { Button, Card, CardTitle, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { fmtNumber } from '../lib/format'
import { errorText, etagOf, fmtDay, fmtMoney, fmtRate, parseMoney, parseNumber, today, toMoneyInput } from '../lib/money'
import { useApp } from '../lib/state'

type CostEntry = Schemas['CostEntry'] & { version: number }
type CostPlan = Schemas['CostPlan'] & { version: number }

export const categoryLabel: Record<string, string> = {
  energy: 'Kraftstoff/Energie', maintenance: 'Wartung', inspection: 'Inspektion', repair: 'Reparatur', upgrade: 'Nachrüstung',
  tax: 'Steuer', insurance: 'Versicherung', fee: 'Gebühren', parking: 'Parken', toll: 'Maut', care: 'Pflege', financing: 'Finanzierung', other: 'Sonstiges',
}
const ownCategories = ['tax', 'insurance', 'fee', 'parking', 'toll', 'care', 'financing', 'other']
const groupLabel = (groupBy: string, key: string) => groupBy === 'category' ? categoryLabel[key] ?? key
  : groupBy === 'cost_kind' ? ({ parts: 'Teile', labor: 'Arbeit', other: 'Sonstiges', none: 'ohne Kostenart' } as Record<string, string>)[key] ?? key
    : groupBy === 'month' ? new Intl.DateTimeFormat('de-DE', { month: 'short', year: 'numeric' }).format(new Date(key + '-15')) : key

export function useCostReport(vid: string | undefined, from: string, to: string, groupBy = 'category', allocation = 'payment') {
  return useQuery({
    enabled: !!vid, queryKey: ['costs', vid, 'report', from, to, groupBy, allocation],
    queryFn: () => api.get<Schemas['CostReport']>(`/vehicles/${vid}/cost-report?from=${from}&to=${to}&group_by=${groupBy}&allocation=${allocation}`),
  })
}

export function CostsPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const qc = useQueryClient()
  const year = new Date().getFullYear()
  const [range, setRange] = useState({ from: `${year}-01-01`, to: today() })
  const [groupBy, setGroupBy] = useState('category')
  const [allocation, setAllocation] = useState('payment')
  const [entryDialog, setEntryDialog] = useState<CostEntry | 'new' | null>(null)
  const [planDialog, setPlanDialog] = useState<CostPlan | 'new' | null>(null)
  const [dismissing, setDismissing] = useState<Schemas['CostOccurrence'] | null>(null)
  const report = useCostReport(vid, range.from, range.to, groupBy, allocation)
  const occ = useQuery({ enabled: !!vid, queryKey: ['costs', vid, 'occurrences'], queryFn: () => api.get<Schemas['CostOccurrenceList']>(`/vehicles/${vid}/cost-occurrences`) })
  const plans = useQuery({ enabled: !!vid, queryKey: ['costs', vid, 'plans'], queryFn: () => api.get<{ items: CostPlan[] }>(`/vehicles/${vid}/cost-plans`) })
  const entries = useQuery({ enabled: !!vid, queryKey: ['costs', vid, 'entries'], queryFn: () => api.get<{ items: CostEntry[] }>(`/vehicles/${vid}/cost-entries?limit=100`) })
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'
  const confirm = useMutation({
    mutationFn: (o: Schemas['CostOccurrence']) => api.post(`/vehicles/${vid}/cost-plans/${o.plan_id}/occurrences/${o.due_on}/confirm`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['costs', vid] }),
  })

  if (!vehicle) return <><Header title="Kosten" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>
  const rep = report.data

  return (
    <>
      <Header title="Kosten" sub={`${vehicle.display_name} · alle Kosten aus Service und sonstigen Ausgaben`}
        actions={canEdit && <div className="flex gap-2"><Button variant="outline" icon="calendar" onClick={() => setPlanDialog('new')}>Wiederkehrend</Button><Button icon="plus" onClick={() => setEntryDialog('new')}>Kosten erfassen</Button></div>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        {(occ.data?.items.length ?? 0) > 0 && (
          <Card className="flex flex-col gap-3 p-5">
            <CardTitle aside={occ.data!.more_open > 0 && <span className="text-[13px] text-muted">{occ.data!.more_open} weitere offen</span>}>Anstehende Kosten</CardTitle>
            {occ.data!.items.map((o) => (
              <div key={o.plan_id + o.due_on} className="flex flex-wrap items-center gap-3 rounded-[12px] bg-soft px-3.5 py-2.5">
                <Chip tone={o.state === 'open' ? 'warn' : 'info'} icon="calendar">{o.state === 'open' ? 'fällig' : 'demnächst'}</Chip>
                <div className="flex min-w-0 flex-grow flex-col"><span className="text-sm font-semibold">{o.title}</span><span className="text-xs text-muted">{fmtDay(o.due_on)}</span></div>
                <span className="tabular text-sm font-semibold">{fmtMoney(o.amount.amount_minor, o.amount.currency)}</span>
                {canEdit && <>
                  <Button variant="navy" disabled={confirm.isPending} onClick={() => confirm.mutate(o)}>Bezahlt</Button>
                  <Button variant="ghost" onClick={() => setDismissing(o)}>Entfällt</Button>
                </>}
              </div>
            ))}
            <div className="text-xs text-muted">Vectra legt keine Einträge im Hintergrund an – erst „Bezahlt“ bucht die Kosten.</div>
          </Card>
        )}

        <Card className="flex flex-col gap-4 p-5">
          <div className="flex flex-wrap items-end gap-3">
            <h2 className="m-0 mr-auto font-display text-base font-semibold">Auswertung</h2>
            <Field label="Von" htmlFor="r-from"><input id="r-from" type="date" className={inputClass + ' h-10'} value={range.from} onChange={(e) => setRange({ ...range, from: e.target.value })} /></Field>
            <Field label="Bis" htmlFor="r-to"><input id="r-to" type="date" className={inputClass + ' h-10'} value={range.to} onChange={(e) => setRange({ ...range, to: e.target.value })} /></Field>
            <Field label="Gruppierung" htmlFor="r-group">
              <select id="r-group" className={inputClass + ' h-10'} value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
                <option value="category">Kategorie</option><option value="cost_kind">Kostenart</option><option value="month">Monat</option><option value="year">Jahr</option>
              </select>
            </Field>
            <Field label="Verteilung" htmlFor="r-alloc">
              <select id="r-alloc" className={inputClass + ' h-10'} value={allocation} onChange={(e) => setAllocation(e.target.value)}>
                <option value="payment">nach Zahlungsdatum</option><option value="prorated">zeitanteilig</option>
              </select>
            </Field>
          </div>
          {rep && rep.currencies.length === 0 && <div className="text-sm text-muted">Im Zeitraum sind keine Kosten erfasst.</div>}
          {rep?.currencies.map((c) => {
            const max = Math.max(1, ...c.groups.map((g) => Math.abs(g.amount_minor)))
            return (
              <div key={c.currency} className="flex flex-col gap-4">
                <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
                  <Stat label="Laufende Kosten" value={fmtMoney(c.running_total_minor, c.currency)} />
                  <Stat label="je km" value={c.per_distance ? fmtRate(c.per_distance.value, c.currency) : '–'} note={rep.distance ? `${fmtNumber(rep.distance.value)} ${rep.distance.unit} gefahren` : 'Strecke unbekannt'} />
                  <Stat label="je Tag" value={c.per_day ? fmtRate(c.per_day.value, c.currency) : '–'} note={rep.ownership_days ? `${rep.ownership_days} Besitztage` : ''} />
                  <Stat label={c.depreciation?.appreciation ? 'Wertsteigerung' : 'Wertverlust'} value={c.depreciation ? fmtMoney(Math.abs(c.depreciation.total_minor), c.currency) : '–'}
                    note={c.tco_minor != null ? `Gesamtkosten ${fmtMoney(c.tco_minor, c.currency)}` : c.depreciation?.estimated ? 'geschätzt' : 'Kauf-/Verkaufspreis fehlt'} />
                </div>
                <div className="flex flex-col gap-2" role="list" aria-label={`Kosten nach ${groupBy}`}>
                  {c.groups.map((g) => (
                    <div key={g.key} role="listitem" className="grid grid-cols-[140px_minmax(0,1fr)_110px] items-center gap-3 text-sm">
                      <span className="truncate">{groupLabel(groupBy, g.key)}</span>
                      <div className="h-2.5 rounded-full bg-soft"><div className="h-2.5 rounded-full bg-teal" style={{ width: `${Math.max(2, (Math.abs(g.amount_minor) / max) * 100)}%` }} /></div>
                      <span className="tabular text-right font-semibold">{fmtMoney(g.amount_minor, c.currency)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
          {(rep?.hints?.length ?? 0) > 0 && <ul className="m-0 flex flex-col gap-1 pl-4 text-xs text-muted">{rep!.hints!.map((h) => <li key={h}>{h}</li>)}</ul>}
        </Card>

        <div className="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <Card className="flex flex-col overflow-hidden">
            <div className="px-5 py-4"><CardTitle>Sonstige Kosten</CardTitle></div>
            {(entries.data?.items.length ?? 0) === 0 && <div className="border-t border-line px-5 py-6 text-sm text-muted">Noch keine Einträge. Werkstattkosten erscheinen automatisch aus der Servicehistorie.</div>}
            {entries.data?.items.map((e) => (
              <button key={e.id} type="button" disabled={!canEdit} onClick={() => setEntryDialog(e)}
                className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-3 text-left text-text hover:bg-soft disabled:hover:bg-transparent">
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-sm font-semibold">{e.title}</span>
                  <span className="text-xs text-muted">{categoryLabel[e.category]} · {fmtDay(e.incurred_on)}{e.covers_from && e.covers_to ? ` · gilt ${fmtDay(e.covers_from)} – ${fmtDay(e.covers_to)}` : ''}</span>
                </div>
                <span className="tabular text-sm font-semibold">{fmtMoney(e.amount.amount_minor, e.amount.currency)}</span>
              </button>
            ))}
          </Card>
          <Card className="flex flex-col overflow-hidden">
            <div className="px-5 py-4"><CardTitle>Wiederkehrende Kosten</CardTitle></div>
            {(plans.data?.items.length ?? 0) === 0 && <div className="border-t border-line px-5 py-6 text-sm text-muted">z. B. Kfz-Steuer jährlich oder Versicherung.</div>}
            {plans.data?.items.map((p) => (
              <button key={p.id} type="button" disabled={!canEdit} onClick={() => setPlanDialog(p)}
                className={`grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-t border-line px-5 py-3 text-left text-text hover:bg-soft disabled:hover:bg-transparent ${p.active === false ? 'opacity-60' : ''}`}>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-sm font-semibold">{p.title}</span>
                  <span className="text-xs text-muted">{p.interval_months ? (p.interval_months === 12 ? 'jährlich' : p.interval_months === 1 ? 'monatlich' : `alle ${p.interval_months} Monate`) : `alle ${p.interval_days} Tage`} ab {fmtDay(p.first_due_on)}</span>
                </div>
                <span className="tabular text-sm font-semibold">{fmtMoney(p.amount.amount_minor, p.amount.currency)}</span>
              </button>
            ))}
          </Card>
        </div>
      </main>
      {entryDialog && <EntryDialog vid={vehicle.id} currency={vehicle.default_currency ?? 'EUR'} entry={entryDialog === 'new' ? null : entryDialog} onClose={() => setEntryDialog(null)} />}
      {planDialog && <PlanDialog vid={vehicle.id} currency={vehicle.default_currency ?? 'EUR'} plan={planDialog === 'new' ? null : planDialog} onClose={() => setPlanDialog(null)} />}
      {dismissing && <DismissDialog vid={vehicle.id} occ={dismissing} onClose={() => setDismissing(null)} />}
    </>
  )
}

function Stat({ label, value, note }: { label: string; value: string; note?: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-[14px] bg-soft p-3.5">
      <div className="text-[13px] font-medium text-muted">{label}</div>
      <div className="tabular font-display text-[22px] font-semibold">{value}</div>
      {note && <div className="text-xs text-muted">{note}</div>}
    </div>
  )
}

function EntryDialog({ vid, currency, entry, onClose }: { vid: string; currency: string; entry: CostEntry | null; onClose: () => void }) {
  const qc = useQueryClient()
  const cur = entry?.amount.currency ?? currency
  const [f, setF] = useState({
    title: entry?.title ?? '', category: entry?.category ?? 'insurance', date: entry?.incurred_on ?? today(), amount: entry ? toMoneyInput(entry.amount.amount_minor, cur) : '',
    currency: cur, from: entry?.covers_from ?? '', to: entry?.covers_to ?? '', note: entry?.note ?? '',
  })
  const [error, setError] = useState<string>()
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const done = () => { qc.invalidateQueries({ queryKey: ['costs', vid] }); onClose() }
  const save = useMutation({
    mutationFn: () => {
      const amount = parseMoney(f.amount, f.currency)
      if (amount === null) throw new Error('Betrag fehlt')
      const body = { title: f.title, category: f.category, incurred_on: f.date, amount: { amount_minor: amount, currency: f.currency },
        covers_from: f.from || null, covers_to: f.to || null, note: f.note }
      return entry ? api.patch(`/vehicles/${vid}/cost-entries/${entry.id}`, body, etagOf(entry)) : api.post(`/vehicles/${vid}/cost-entries`, body)
    },
    onSuccess: done, onError: (e) => setError(e instanceof Error && e.message === 'Betrag fehlt' ? 'Bitte einen Betrag angeben.' : errorText(e)),
  })
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vid}/cost-entries/${entry!.id}`, etagOf(entry!)), onSuccess: done, onError: (e) => setError(errorText(e)) })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={entry ? 'Kosten bearbeiten' : 'Kosten erfassen'}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); save.mutate() }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Bezeichnung" htmlFor="e-title"><input id="e-title" required className={inputClass} value={f.title} onChange={set('title')} placeholder="Haftpflicht 2026" /></Field>
          <Field label="Kategorie" htmlFor="e-cat">
            <select id="e-cat" className={inputClass} value={f.category} onChange={set('category')}>{ownCategories.map((k) => <option key={k} value={k}>{categoryLabel[k]}</option>)}</select>
          </Field>
          <Field label="Bezahlt am" htmlFor="e-date"><input id="e-date" type="date" required className={inputClass} value={f.date} onChange={set('date')} /></Field>
          <Field label={`Betrag (${f.currency})`} htmlFor="e-amount"><input id="e-amount" inputMode="decimal" required className={inputClass + ' tabular'} value={f.amount} onChange={set('amount')} placeholder="0,00" /></Field>
          <Field label="Gilt von" htmlFor="e-from" hint="optional, für die zeitanteilige Ansicht"><input id="e-from" type="date" className={inputClass} value={f.from} onChange={set('from')} /></Field>
          <Field label="Gilt bis" htmlFor="e-to"><input id="e-to" type="date" className={inputClass} value={f.to} onChange={set('to')} /></Field>
        </div>
        <Field label="Notiz" htmlFor="e-note"><input id="e-note" className={inputClass} value={f.note} onChange={set('note')} /></Field>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <div className="flex flex-wrap justify-between gap-3">
          {entry ? <Button type="button" variant="outline" disabled={remove.isPending} onClick={() => remove.mutate()}>Löschen</Button> : <span />}
          <Button type="submit" disabled={save.isPending}>Speichern</Button>
        </div>
      </form>
    </Dialog>
  )
}

function PlanDialog({ vid, currency, plan, onClose }: { vid: string; currency: string; plan: CostPlan | null; onClose: () => void }) {
  const qc = useQueryClient()
  const cur = plan?.amount.currency ?? currency
  const [f, setF] = useState({
    title: plan?.title ?? '', category: plan?.category ?? 'tax', amount: plan ? toMoneyInput(plan.amount.amount_minor, cur) : '', months: String(plan?.interval_months ?? 12),
    first: plan?.first_due_on ?? today(), ends: plan?.ends_on ?? '', remind: String(plan?.remind_days_before ?? 30), active: plan?.active !== false,
  })
  const [error, setError] = useState<string>()
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const done = () => { qc.invalidateQueries({ queryKey: ['costs', vid] }); onClose() }
  const save = useMutation({
    mutationFn: () => {
      const body = { title: f.title, category: f.category, amount: { amount_minor: parseMoney(f.amount, cur) ?? 0, currency: cur }, interval_months: parseNumber(f.months),
        interval_days: null, first_due_on: f.first, ends_on: f.ends || null, remind_days_before: parseNumber(f.remind) ?? 30, active: f.active }
      return plan ? api.patch(`/vehicles/${vid}/cost-plans/${plan.id}`, body, etagOf(plan)) : api.post(`/vehicles/${vid}/cost-plans`, body)
    },
    onSuccess: done, onError: (e) => setError(errorText(e)),
  })
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vid}/cost-plans/${plan!.id}`, etagOf(plan!)), onSuccess: done, onError: (e) => setError(errorText(e)) })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={plan ? 'Wiederkehrende Kosten bearbeiten' : 'Wiederkehrende Kosten'}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); save.mutate() }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Bezeichnung" htmlFor="p-title"><input id="p-title" required className={inputClass} value={f.title} onChange={set('title')} placeholder="Kfz-Steuer" /></Field>
          <Field label="Kategorie" htmlFor="p-cat">
            <select id="p-cat" className={inputClass} value={f.category} onChange={set('category')}>{ownCategories.map((k) => <option key={k} value={k}>{categoryLabel[k]}</option>)}</select>
          </Field>
          <Field label={`Betrag (${cur})`} htmlFor="p-amount"><input id="p-amount" inputMode="decimal" required className={inputClass + ' tabular'} value={f.amount} onChange={set('amount')} /></Field>
          <Field label="Alle … Monate" htmlFor="p-months"><input id="p-months" inputMode="numeric" required className={inputClass} value={f.months} onChange={set('months')} /></Field>
          <Field label="Erste Fälligkeit" htmlFor="p-first"><input id="p-first" type="date" required className={inputClass} value={f.first} onChange={set('first')} /></Field>
          <Field label="Endet am" htmlFor="p-ends" hint="optional"><input id="p-ends" type="date" className={inputClass} value={f.ends} onChange={set('ends')} /></Field>
          <Field label="Erinnern … Tage vorher" htmlFor="p-remind"><input id="p-remind" inputMode="numeric" className={inputClass} value={f.remind} onChange={set('remind')} /></Field>
        </div>
        {plan && <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="h-4 w-4 accent-teal" checked={f.active} onChange={(e) => setF({ ...f, active: e.target.checked })} />Aktiv</label>}
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        <div className="flex flex-wrap justify-between gap-3">
          {plan ? <Button type="button" variant="outline" disabled={remove.isPending} onClick={() => remove.mutate()}>Löschen</Button> : <span />}
          <Button type="submit" disabled={save.isPending}>Speichern</Button>
        </div>
      </form>
    </Dialog>
  )
}

function DismissDialog({ vid, occ, onClose }: { vid: string; occ: Schemas['CostOccurrence']; onClose: () => void }) {
  const qc = useQueryClient()
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string>()
  const save = useMutation({
    mutationFn: () => api.post(`/vehicles/${vid}/cost-plans/${occ.plan_id}/occurrences/${occ.due_on}/dismiss`, { reason }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['costs', vid] }); onClose() }, onError: (e) => setError(errorText(e)),
  })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={`${occ.title} entfällt`}>
      <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); save.mutate() }}>
        <Field label="Begründung" htmlFor="d-reason" error={error}><input id="d-reason" required className={inputClass} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="z. B. Steuerbefreiung" /></Field>
        <Button type="submit" className="self-end" disabled={save.isPending || !reason.trim()}><Icon name="check" size={18} />Bestätigen</Button>
      </form>
    </Dialog>
  )
}
