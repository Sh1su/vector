import { Fragment, useEffect, useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, streamPost, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Card, Chip, EmptyState, inputClass } from '../components/ui'
import { carrierLabel, fmtDate, fmtNumber } from '../lib/format'
import { problemText } from '../lib/mutation'
import { useApp, type Account } from '../lib/state'

type Status = Schemas['AssistantStatus']
type Message = Schemas['AssistantMessage']
type Proposal = Schemas['Proposal']

const toolLabel: Record<string, string> = {
  list_vehicles: 'Fahrzeuge', get_vehicle: 'Fahrzeugdaten', get_current_odometer: 'Kilometerstand', get_fuel_consumption: 'Verbrauch',
  get_oil_consumption: 'Ölverbrauch', get_due_maintenance: 'Wartungen', get_vehicle_history: 'Verlauf',
  create_odometer_entry: 'Vorschlag Kilometerstand', create_fuel_fill: 'Vorschlag Tankvorgang', create_oil_entry: 'Vorschlag Öleintrag', create_maintenance_event: 'Vorschlag Wartung',
}

const opLabel: Record<string, string> = { createOdometerReading: 'Kilometerstand erfassen', createFuelFill: 'Tankvorgang erfassen', createOilEntry: 'Öleintrag erfassen', createCompletion: 'Wartung erledigen' }
const kindLabel: Record<string, string> = { check: 'Messung', top_up: 'Nachfüllung', oil_change: 'Ölwechsel', done: 'erledigt', skipped: 'ausgelassen' }

export function useAssistantStatus() {
  return useQuery({ queryKey: ['assistant', 'status'], queryFn: () => api.get<Status>('/assistant/status') })
}

export function AssistantPage() {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.get<Account>('/me') }).data
  const status = useAssistantStatus()
  const st = status.data
  if (status.isPending) return <><Header title="Assistent" /><main className="p-8 text-muted">Lade …</main></>
  if (!st?.enabled) {
    return (
      <>
        <Header title="Assistent" />
        <main className="p-8">
          <EmptyState icon="sparkles" title="Der Assistent ist ausgeschaltet"
            text={me?.is_admin
              ? st?.chat_provider ? `Anbieter „${st.chat_provider.name}“ ist konfiguriert. Schalte den Assistenten unter Einstellungen → Installation ein.` : 'Es ist noch kein Sprachmodell konfiguriert. Setze VECTRA_ASSISTANT_PROVIDER und VECTRA_ASSISTANT_MODEL (siehe deploy/README.md) und schalte ihn danach unter Einstellungen ein.'
              : 'Der Assistent ist optional. Bitte wende dich an die Administration der Installation. Vectra ist ohne ihn voll nutzbar.'}
            action={me?.is_admin && st?.chat_provider ? <Link to="/einstellungen" className="text-sm font-semibold text-link">Zu den Einstellungen</Link> : undefined} />
        </main>
      </>
    )
  }
  if (!st.user_enabled) return <><Header title="Assistent" /><main className="p-8"><ConsentCard status={st} /></main></>
  return <Chat status={st} />
}

export function ConsentCard({ status }: { status: Status }) {
  const qc = useQueryClient()
  const [accept, setAccept] = useState(!status.chat_provider?.external)
  const [error, setError] = useState<string>()
  const external = status.chat_provider?.external
  async function enable() {
    setError(undefined)
    try {
      await api.post('/assistant/consent', { accept_external_provider: accept, provider_name: status.chat_provider?.name ?? '' })
      qc.invalidateQueries({ queryKey: ['assistant'] })
    } catch (e) { setError(problemText(e)) }
  }
  return (
    <Card className="flex max-w-2xl flex-col gap-4 p-6">
      <div className="flex items-center gap-3"><div className="flex h-11 w-11 items-center justify-center rounded-[14px] bg-info-bg text-link"><Icon name="sparkles" size={24} /></div>
        <div className="font-display text-lg font-semibold">Assistent einschalten</div></div>
      <p className="m-0 text-sm leading-relaxed">Der Assistent beantwortet Fragen zu deinen Fahrzeugen und bereitet Einträge vor („Ich habe bei 143.520 km 0,7 Liter Öl nachgefüllt“). Er speichert <b>nie</b> selbst: Jeden Vorschlag bestätigst du.</p>
      {external ? (
        <label className="flex items-start gap-2.5 rounded-[12px] border-[1.5px] border-amber bg-warn-bg p-3.5 text-sm">
          <input type="checkbox" className="mt-0.5 h-4 w-4 accent-teal" checked={accept} onChange={(e) => setAccept(e.target.checked)} />
          <span>Ich stimme zu, dass meine Fragen und die dafür nötigen Fahrzeugdaten an <b>{status.chat_provider?.name}</b> (externer Dienst) übertragen werden. FIN, Notizen und Daten anderer Mitglieder werden nicht übertragen.</span>
        </label>
      ) : (
        <div className="text-sm text-muted">Anbieter: {status.chat_provider?.name} (lokal, keine Übertragung an Dritte).</div>
      )}
      {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
      <Button type="button" onClick={enable} disabled={!accept} className="self-start">Einschalten</Button>
    </Card>
  )
}

function Chat({ status }: { status: Status }) {
  const { vehicle } = useApp()
  const qc = useQueryClient()
  const convs = useQuery({ queryKey: ['assistant', 'conversations'], queryFn: () => api.get<{ items: Schemas['Conversation'][] }>('/assistant/conversations') })
  const [convId, setConvId] = useState<string | null>(null)
  const messages = useQuery({ enabled: !!convId, queryKey: ['assistant', 'messages', convId],
    queryFn: () => api.get<{ items: Message[] }>(`/assistant/conversations/${convId}/messages`) })
  const [text, setText] = useState('')
  const [pending, setPending] = useState<{ text: string; tools: string[] } | null>(null)
  const [error, setError] = useState<string>()
  const endRef = useRef<HTMLDivElement>(null)
  useEffect(() => { endRef.current?.scrollIntoView({ block: 'end' }) }, [messages.data, pending])

  async function send(e: FormEvent) {
    e.preventDefault()
    const t = text.trim()
    if (!t || pending) return
    setError(undefined)
    let id = convId
    try {
      if (!id) {
        const c = await api.post<Schemas['Conversation']>('/assistant/conversations', { vehicle_id: vehicle?.id ?? null })
        id = c.id!
        setConvId(id)
      }
      setText('')
      setPending({ text: t, tools: [] })
      await streamPost(`/assistant/conversations/${id}/messages`, { text: t }, (ev, data) => {
        if (ev === 'tool') setPending((p) => p && { ...p, tools: [...p.tools, (data as { name: string }).name] })
        if (ev === 'error') setError((data as { title?: string; detail?: string }).detail || (data as { title?: string }).title || 'Fehler')
      })
    } catch (err) {
      setError(err instanceof ProblemError ? problemText(err) : 'Senden fehlgeschlagen.')
    } finally {
      setPending(null)
      qc.invalidateQueries({ queryKey: ['assistant', 'messages', id] })
      qc.invalidateQueries({ queryKey: ['assistant', 'conversations'] })
    }
  }

  async function remove(id: string) {
    if (!confirm('Unterhaltung löschen?')) return
    await api.del(`/assistant/conversations/${id}`, '*')
    if (id === convId) setConvId(null)
    qc.invalidateQueries({ queryKey: ['assistant', 'conversations'] })
  }

  const list = messages.data?.items ?? []
  return (
    <>
      <Header title="Assistent" sub={`Antworten werden von ${status.chat_provider?.name} erzeugt${status.chat_provider?.external ? ' (externer Dienst)' : ''}`}
        actions={<Button variant="outline" icon="plus" onClick={() => setConvId(null)}>Neue Unterhaltung</Button>} />
      <div className="flex min-h-0 flex-grow">
        <aside className="hidden w-[260px] shrink-0 flex-col gap-1 overflow-y-auto border-r border-line bg-card p-3 lg:flex" aria-label="Unterhaltungen">
          {(convs.data?.items ?? []).map((c) => (
            <div key={c.id} className={`group flex items-center gap-1 rounded-[10px] ${c.id === convId ? 'bg-info-bg' : 'hover:bg-soft'}`}>
              <button type="button" onClick={() => setConvId(c.id!)} className="min-w-0 flex-grow truncate px-2.5 py-2 text-left text-sm">{c.title || 'Neue Unterhaltung'}</button>
              <button type="button" aria-label="Löschen" onClick={() => remove(c.id!)} className="mr-1 hidden h-7 w-7 items-center justify-center rounded-[8px] text-muted group-hover:flex hover:bg-card"><Icon name="x" size={16} /></button>
            </div>
          ))}
          {(convs.data?.items ?? []).length === 0 && <div className="px-2 py-2 text-xs text-muted">Noch keine Unterhaltungen. Inhalte werden nach der Aufbewahrungsfrist gelöscht.</div>}
        </aside>
        <main className="flex min-w-0 flex-grow flex-col">
          <div className="flex min-h-0 flex-grow flex-col gap-4 overflow-y-auto px-8 py-6">
            {list.length === 0 && !pending && (
              <div className="flex flex-col gap-3">
                <div className="text-sm text-muted">Beispiele{vehicle ? ` für ${vehicle.display_name}` : ''}:</div>
                {['Wie hoch ist mein Durchschnittsverbrauch?', 'Was ist als Nächstes an Wartung fällig?', 'Ich habe bei 143.520 km 0,7 Liter Öl nachgefüllt.', 'Heute vollgetankt: 62,4 l Diesel für 104,20 € bei 98.300 km.'].map((x) => (
                  <button key={x} type="button" onClick={() => setText(x)} className="self-start rounded-[12px] border border-line bg-card px-3.5 py-2 text-left text-sm hover:border-teal">{x}</button>
                ))}
              </div>
            )}
            {list.map((m) => <Bubble key={m.id} m={m} />)}
            {pending && <>
              <Bubble m={{ role: 'user', text: pending.text } as Message} />
              <div className="flex items-center gap-2 text-sm text-muted"><Icon name="sparkles" size={18} className="animate-pulse text-link" />
                {pending.tools.length ? `prüft: ${pending.tools.map((t) => toolLabel[t] ?? t).join(', ')} …` : 'denkt nach …'}</div>
            </>}
            {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
            <div ref={endRef} />
          </div>
          <form onSubmit={send} className="flex gap-3 border-t border-line bg-card px-8 py-4">
            <label htmlFor="ask" className="sr-only">Nachricht</label>
            <input id="ask" className={inputClass} value={text} onChange={(e) => setText(e.target.value)} placeholder="Frag nach Verbrauch, Wartung oder sag, was du erfassen willst …" maxLength={8000} />
            <Button type="submit" icon="right" disabled={!text.trim() || !!pending}>Senden</Button>
          </form>
        </main>
      </div>
    </>
  )
}

function Bubble({ m }: { m: Message }) {
  const user = m.role === 'user'
  return (
    <div className={`flex flex-col gap-2 ${user ? 'items-end' : 'items-start'}`}>
      <div className={`max-w-[720px] rounded-[16px] px-4 py-3 text-[15px] leading-relaxed whitespace-pre-wrap ${user ? 'bg-navy text-white' : 'border border-line bg-card'}`}>{m.text}</div>
      {(m.proposals ?? []).map((p) => <ProposalCard key={p.id} p={p} />)}
    </div>
  )
}

function describe(p: Proposal) {
  const b = p.body as Record<string, unknown>
  const q = (x: unknown) => { const v = x as { value?: number; unit?: string } | undefined; return v?.value != null ? `${fmtNumber(v.value, 2)} ${v.unit ?? ''}` : null }
  const lines: [string, string | null][] = [
    ['Zeitpunkt', typeof b.occurred_at === 'string' ? fmtDate(b.occurred_at) : typeof b.completed_on === 'string' ? fmtDate(b.completed_on, true) : null],
    ['Art', typeof b.kind === 'string' ? kindLabel[b.kind] ?? b.kind : null],
    ['Energieträger', typeof b.energy_carrier === 'string' ? carrierLabel[b.energy_carrier] : null],
    ['Menge', q(b.quantity) ?? q(b.oil_added) ?? q(b.oil_change_fill)],
    ['Stand', q(b.odometer) ?? q(b.value) ?? q(b.completed_odometer)],
    ['Betrag', b.cost ? `${fmtNumber((b.cost as { amount_minor: number }).amount_minor / 100, 2)} ${(b.cost as { currency: string }).currency}` : null],
    ['Füllung', b.fill_level === 'full' ? 'voll' : b.fill_level === 'partial' ? 'teilweise' : null],
    ['Begründung', typeof b.reason === 'string' ? b.reason : null],
  ]
  return lines.filter(([, v]) => v)
}

function ProposalCard({ p }: { p: Proposal }) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  const [anomalies, setAnomalies] = useState(p.anomalies?.length ? p.anomalies : null)
  const [error, setError] = useState<string>()
  const [editing, setEditing] = useState(false)
  const [json, setJson] = useState(() => JSON.stringify(p.body, null, 2))
  const done = () => { qc.invalidateQueries({ queryKey: ['assistant', 'messages'] }); qc.invalidateQueries({ queryKey: ['odometer'] }); qc.invalidateQueries({ queryKey: ['fuel'] }); qc.invalidateQueries({ queryKey: ['oil'] }); qc.invalidateQueries({ queryKey: ['maintenance'] }) }
  async function confirmIt(codes?: string[], reason?: string) {
    setBusy(true)
    setError(undefined)
    try {
      const body: Record<string, unknown> = {}
      if (editing) body.body = JSON.parse(json)
      if (codes) { body.confirm_anomalies = codes; body.anomaly_reason = reason }
      await api.post(`/assistant/proposals/${p.id}/confirm`, body, { 'Idempotency-Key': p.id })
      done()
    } catch (e) {
      if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
      else setError(e instanceof SyntaxError ? 'Ungültiges JSON.' : problemText(e))
    } finally { setBusy(false) }
  }
  async function reject() {
    setBusy(true)
    try { await api.post(`/assistant/proposals/${p.id}/reject`); done() } finally { setBusy(false) }
  }
  const state = { confirmed: { tone: 'ok' as const, label: 'gespeichert' }, rejected: { tone: 'neutral' as const, label: 'verworfen' }, expired: { tone: 'neutral' as const, label: 'abgelaufen' }, pending: { tone: 'info' as const, label: 'wartet auf Bestätigung' } }[p.status]
  return (
    <Card className="flex w-full max-w-[560px] flex-col gap-3 p-4">
      <div className="flex items-center justify-between gap-2"><div className="flex items-center gap-2 font-semibold"><Icon name="sparkles" size={18} className="text-link" />{opLabel[p.operation] ?? p.operation}</div><Chip tone={state.tone}>{state.label}</Chip></div>
      {!editing && <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        {describe(p).map(([k, v]) => <Fragment key={k}><dt className="text-muted">{k}</dt><dd className="m-0 tabular">{v}</dd></Fragment>)}
      </dl>}
      {editing && <textarea aria-label="Vorschlag bearbeiten" className="min-h-[180px] rounded-[12px] border-[1.5px] border-line bg-card p-3 font-mono text-xs" value={json} onChange={(e) => setJson(e.target.value)} />}
      {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
      {p.status === 'pending' && (anomalies
        ? <PlausibilityAlert anomalies={anomalies} busy={busy} onEdit={() => { setAnomalies(null); setEditing(true) }} onConfirm={(c, r) => confirmIt(c, r)} />
        : <div className="flex flex-wrap gap-2">
          <Button type="button" icon="check" disabled={busy} onClick={() => confirmIt()}>Bestätigen</Button>
          <Button type="button" variant="outline" icon="edit" disabled={busy} onClick={() => setEditing(!editing)}>{editing ? 'Ansicht' : 'Bearbeiten'}</Button>
          <Button type="button" variant="ghost" disabled={busy} onClick={reject}>Verwerfen</Button>
        </div>)}
    </Card>
  )
}
