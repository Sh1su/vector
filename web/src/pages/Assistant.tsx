import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { Link } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, request, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Button, Card, Chip, EmptyState, Field } from '../components/ui'
import { errorText } from '../lib/money'
import { useApp } from '../lib/state'

type Status = Schemas['AssistantStatus']
type Conversation = Schemas['Conversation']
type Message = Schemas['AssistantMessage']
type Proposal = Schemas['Proposal']

const examples = [
  'Ich fahre jetzt los, Kilometerstand 52.340, Kundentermin bei Müller in Potsdam',
  'Bin angekommen, Stand 52.398',
  'Lade die Wartungsintervalle für meinen Skoda Octavia nach',
  'Was ist als Nächstes fällig?',
]

function csrf() {
  const m = document.cookie.match(/(?:^|;\s*)vectra_csrf=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : ''
}

/** Nachricht senden; die Antwort kommt als Server-Sent Events (status, proposal, message, error). */
async function sendMessage(convId: string, text: string, onEvent: (event: string, data: unknown) => void) {
  const res = await fetch(`/api/v1/assistant/conversations/${convId}/messages`, {
    method: 'POST', credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream', 'X-CSRF-Token': csrf() },
    body: JSON.stringify({ text }),
  })
  if (!res.ok || !res.body) {
    const json = await res.json().catch(() => ({ status: res.status, title: res.statusText, type: 'about:blank' }))
    throw new ProblemError(json)
  }
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
  let buf = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buf += value
    let i: number
    while ((i = buf.indexOf('\n\n')) >= 0) {
      const chunk = buf.slice(0, i)
      buf = buf.slice(i + 2)
      let event = 'message'
      let data = ''
      for (const line of chunk.split('\n')) {
        if (line.startsWith('event: ')) event = line.slice(7)
        else if (line.startsWith('data: ')) data += line.slice(6)
      }
      if (data) onEvent(event, JSON.parse(data))
    }
  }
}

// Web Speech API (Chrome, Edge, Safari); Firefox hat sie nicht – dort hilft ein Diktierprogramm wie Wispr Flow.
type Recognition = { lang: string; interimResults: boolean; continuous: boolean; start(): void; stop(): void
  onresult: ((e: { results: ArrayLike<ArrayLike<{ transcript: string }> & { isFinal: boolean }> }) => void) | null
  onend: (() => void) | null; onerror: ((e: { error: string }) => void) | null }
const SpeechRecognition = (window as unknown as { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition }).SpeechRecognition
  ?? (window as unknown as { webkitSpeechRecognition?: new () => Recognition }).webkitSpeechRecognition

export function AssistantPage() {
  const status = useQuery({ queryKey: ['assistant', 'status'], queryFn: () => api.get<Status>('/assistant/status') })
  const s = status.data
  return (
    <>
      <Header title="Assistent" sub={s?.enabled ? `Antworten erzeugt von ${s.chat_provider?.name ?? 'Anthropic (Claude)'}${s.model ? ' · ' + s.model : ''}` : 'Mit Vectra sprechen und Einträge diktieren'} />
      {status.isPending ? <main className="p-8 text-muted">Lade …</main>
        : !s?.enabled ? <NotConfigured />
          : s.consent_required ? <Consent status={s} />
            : <Chat status={s} />}
    </>
  )
}

function NotConfigured() {
  return (
    <main className="flex flex-col gap-5 p-8">
      <EmptyState icon="sparkles" title="Der Assistent ist nicht eingerichtet"
        text="Ein Administrator aktiviert ihn mit VECTRA_ASSISTANT_PROVIDER=anthropic, VECTRA_ASSISTANT_MODEL=claude-sonnet-4-5 und einem ANTHROPIC_API_KEY (siehe deploy/.env.example)." />
      <McpHint />
    </main>
  )
}

function McpHint() {
  return (
    <div className="flex items-start gap-2.5 rounded-[12px] bg-info-bg px-3.5 py-3 text-[13px] leading-normal">
      <span className="text-link"><Icon name="key" size={18} /></span>
      <div>Claude Desktop, Claude Code und andere KI-Clients verbinden sich über den MCP-Server von Vectra (<code>/api/v1/mcp</code>). Das API-Token dafür legst du unter <Link className="font-semibold text-link" to="/einstellungen">Einstellungen → KI-Zugang</Link> an.</div>
    </div>
  )
}

function Consent({ status }: { status: Status }) {
  const qc = useQueryClient()
  const name = status.chat_provider?.name ?? 'Anthropic (Claude)'
  const give = useMutation({
    mutationFn: () => api.post('/assistant/consent', { accept_external_provider: true, provider_name: name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['assistant'] }),
  })
  return (
    <main className="flex justify-center p-8">
      <Card className="flex max-w-[640px] flex-col gap-4 p-6">
        <div className="flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-[12px] bg-info-bg text-link"><Icon name="sparkles" /></div>
          <h2 className="m-0 font-display text-lg font-semibold">Assistent einschalten</h2>
        </div>
        <p className="m-0 text-sm leading-relaxed">Der Assistent nutzt <b>{name}</b>{status.model ? ` (${status.model})` : ''}. Für jede Frage gehen deine Nachricht und die Daten, die er dafür nachschlägt (z. B. Kilometerstand, Fahrten, Wartungen), an diesen Anbieter. Kennzeichen, Standorte aus Fotos und Daten anderer Mitglieder bleiben in Vectra.</p>
        <ul className="m-0 flex list-disc flex-col gap-1.5 pl-5 text-sm leading-relaxed">
          <li>Einträge speichert der Assistent nie selbst: Er bereitet sie vor, du bestätigst mit einem Klick.</li>
          {status.web_search && <li>Für Wartungsintervalle anderer Modelle darf er im Web nach Herstellerangaben suchen – ohne persönliche Daten.</li>}
          <li>Unterhaltungen werden nach 30 Tagen ohne Aktivität gelöscht; du kannst sie jederzeit selbst löschen.</li>
        </ul>
        {give.error && <div role="alert" className="text-sm font-semibold text-bad">{errorText(give.error)}</div>}
        <Button className="self-end" icon="check" disabled={give.isPending} onClick={() => give.mutate()}>Zustimmen und starten</Button>
      </Card>
    </main>
  )
}

function Chat({ status }: { status: Status }) {
  const qc = useQueryClient()
  const { vehicle } = useApp()
  const convs = useQuery({ queryKey: ['assistant', 'conversations'], queryFn: () => api.get<{ items: Conversation[] }>('/assistant/conversations') })
  const [convId, setConvId] = useState<string | null>(null)
  const active = convId ?? convs.data?.items[0]?.id ?? null
  const msgs = useQuery({ enabled: !!active, queryKey: ['assistant', 'messages', active], queryFn: () => api.get<{ items: Message[] }>(`/assistant/conversations/${active}/messages`) })
  const [pending, setPending] = useState<{ text: string; status: string; proposals: Proposal[] } | null>(null)
  const [error, setError] = useState<string>()
  const [text, setText] = useState('')
  const scroller = useRef<HTMLDivElement>(null)

  useEffect(() => { scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' }) }, [msgs.data, pending])

  const newConv = async () => {
    const c = await api.post<Conversation>('/assistant/conversations', { vehicle_id: vehicle?.id ?? null })
    await qc.invalidateQueries({ queryKey: ['assistant', 'conversations'] })
    setConvId(c.id!)
    return c.id!
  }

  const send = async (t: string) => {
    const msg = t.trim()
    if (!msg || pending) return
    setError(undefined)
    setText('')
    let id = active
    try {
      if (!id) id = await newConv()
      setPending({ text: msg, status: 'Denke nach …', proposals: [] })
      await sendMessage(id, msg, (event, data) => {
        if (event === 'status') setPending((p) => p && { ...p, status: (data as { text: string }).text })
        if (event === 'proposal') setPending((p) => p && { ...p, proposals: [...p.proposals, data as Proposal] })
        if (event === 'error') setError(errorText(new ProblemError(data as Schemas['Problem'])))
      })
    } catch (e) {
      setError(errorText(e))
      setText(msg)
    } finally {
      setPending(null)
      qc.invalidateQueries({ queryKey: ['assistant'] })
    }
  }

  const remove = useMutation({
    mutationFn: (id: string) => request('DELETE', `/assistant/conversations/${id}`),
    onSuccess: () => { setConvId(null); qc.invalidateQueries({ queryKey: ['assistant'] }) },
  })

  const list = msgs.data?.items ?? []
  return (
    <main className="flex min-h-0 flex-grow">
      <aside className="hidden w-[260px] shrink-0 flex-col gap-2 border-r border-line bg-card p-4 lg:flex">
        <Button icon="plus" onClick={() => { setConvId(null); void newConv() }}>Neue Unterhaltung</Button>
        <div className="mt-2 flex min-h-0 flex-col gap-1 overflow-y-auto">
          {(convs.data?.items ?? []).map((c) => (
            <div key={c.id} className={`group flex items-center gap-1 rounded-[10px] ${c.id === active ? 'bg-info-bg text-link' : 'hover:bg-soft'}`}>
              <button type="button" onClick={() => setConvId(c.id!)} className="min-w-0 flex-grow truncate px-3 py-2 text-left text-sm font-semibold">
                {c.title || 'Neue Unterhaltung'}
              </button>
              <button type="button" aria-label="Unterhaltung löschen" onClick={() => remove.mutate(c.id!)}
                className="mr-1 hidden h-7 w-7 items-center justify-center rounded-[8px] text-muted hover:bg-card group-hover:flex"><Icon name="x" size={16} /></button>
            </div>
          ))}
        </div>
      </aside>
      <section className="flex min-w-0 flex-grow flex-col">
        <div ref={scroller} className="flex min-h-0 flex-grow flex-col gap-4 overflow-y-auto px-8 py-6">
          {list.length === 0 && !pending && (
            <div className="mx-auto flex max-w-[680px] flex-col gap-4 py-8">
              <EmptyState icon="sparkles" title="Was möchtest du eintragen oder wissen?"
                text={`Sprich oder schreib einfach – z. B. für das Fahrtenbuch. Einträge bereite ich vor, du bestätigst sie.${vehicle ? ` Aktuelles Fahrzeug: ${vehicle.display_name}.` : ''}`} />
              <div className="flex flex-wrap justify-center gap-2">
                {examples.map((e) => (
                  <button key={e} type="button" onClick={() => setText(e)} className="rounded-full border border-line bg-card px-3.5 py-2 text-left text-[13px] hover:bg-soft">{e}</button>
                ))}
              </div>
            </div>
          )}
          {list.map((m) => <Bubble key={m.id} m={m} />)}
          {pending && (
            <>
              <Bubble m={{ role: 'user', text: pending.text }} />
              <div className="flex max-w-[760px] flex-col gap-2">
                <div className="flex items-center gap-2 text-sm text-muted"><span className="h-2 w-2 animate-pulse rounded-full bg-teal" />{pending.status}</div>
                {pending.proposals.map((p) => <ProposalCard key={p.id} p={p} />)}
              </div>
            </>
          )}
          {error && <div role="alert" className="max-w-[760px] rounded-[12px] bg-bad-bg px-3.5 py-3 text-sm font-semibold text-bad">{error}</div>}
        </div>
        <Composer text={text} setText={setText} busy={!!pending} onSend={send} />
        <div className="px-8 pb-3 text-[11px] text-muted">
          Antworten erzeugt von {status.chat_provider?.name ?? 'Anthropic (Claude)'} – sie können falsch sein. Gespeichert wird nur, was du bestätigst.
        </div>
      </section>
    </main>
  )
}

function Bubble({ m }: { m: Pick<Message, 'role' | 'text' | 'proposals'> }) {
  const mine = m.role === 'user'
  return (
    <div className={`flex flex-col gap-2 ${mine ? 'items-end' : 'items-start'}`}>
      <div className={`max-w-[760px] rounded-[16px] px-4 py-3 text-[15px] leading-relaxed whitespace-pre-wrap ${mine ? 'bg-navy text-white' : 'border border-line bg-card'}`}>{m.text}</div>
      {!mine && (m.proposals ?? []).map((p) => <ProposalCard key={p.id} p={p} />)}
    </div>
  )
}

const statusChip: Record<string, { label: string; tone: 'ok' | 'warn' | 'bad' | 'info' | 'neutral' }> = {
  pending: { label: 'Bitte bestätigen', tone: 'warn' }, confirmed: { label: 'Gespeichert', tone: 'ok' },
  rejected: { label: 'Verworfen', tone: 'neutral' }, expired: { label: 'Abgelaufen', tone: 'neutral' },
}

/** Vorschlag (ADR-026): Bestätigen oder Verwerfen; bei Befunden mit Begründung (ADR-010). */
function ProposalCard({ p }: { p: Proposal }) {
  const qc = useQueryClient()
  const [state, setState] = useState(p)
  const [reason, setReason] = useState('')
  const [anomalies, setAnomalies] = useState<Schemas['Anomaly'][]>(p.anomalies ?? [])
  const [error, setError] = useState<string>()
  useEffect(() => setState(p), [p])
  const confirmable = anomalies.length > 0 && anomalies.every((a) => a.confirmable)
  const confirm = useMutation({
    mutationFn: () => api.post<Proposal>(`/assistant/proposals/${p.id}/confirm`, anomalies.length
      ? { confirm_anomalies: anomalies.map((a) => a.code), anomaly_reason: reason.trim() } : {}),
    onSuccess: (r) => { setState(r); setAnomalies([]); qc.invalidateQueries() },
    onError: (e) => {
      if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies ?? [])
      else setError(errorText(e))
    },
  })
  const reject = useMutation({ mutationFn: () => api.post<Proposal>(`/assistant/proposals/${p.id}/reject`), onSuccess: setState, onError: (e) => setError(errorText(e)) })
  const st = statusChip[state.status]
  return (
    <Card className="flex w-full max-w-[560px] flex-col gap-3 p-4">
      <div className="flex items-start gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] bg-info-bg text-link"><Icon name={p.operation.includes('Trip') ? 'route' : p.operation.includes('Maint') ? 'wrench' : p.operation.includes('Cost') ? 'euro' : 'gauge'} /></div>
        <div className="flex min-w-0 flex-grow flex-col gap-1">
          <div className="text-[15px] font-semibold">{state.summary ?? p.operation}</div>
          <div><Chip tone={st.tone}>{st.label}</Chip></div>
        </div>
      </div>
      {anomalies.length > 0 && state.status === 'pending' && (
        <div className="flex flex-col gap-2 rounded-[12px] bg-warn-bg px-3.5 py-3 text-[13px] text-warn">
          {anomalies.map((a) => <div key={a.code}><b>{a.code}</b> {a.message}</div>)}
          {confirmable
            ? <Field label="Begründung, falls der Wert stimmt" htmlFor={`r-${p.id}`}><input id={`r-${p.id}`} className="h-10 w-full rounded-[10px] border border-line bg-card px-3 text-sm text-text" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="z. B. Tacho getauscht" /></Field>
            : <div>Dieser Wert lässt sich so nicht speichern. Bitte im Chat korrigieren.</div>}
        </div>
      )}
      {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
      {state.status === 'pending' && (
        <div className="flex justify-end gap-2">
          <Button variant="outline" disabled={reject.isPending || confirm.isPending} onClick={() => reject.mutate()}>Verwerfen</Button>
          <Button icon="check" disabled={confirm.isPending || (anomalies.length > 0 && (!confirmable || !reason.trim()))} onClick={() => { setError(undefined); confirm.mutate() }}>
            {anomalies.length ? 'Trotzdem speichern' : 'Bestätigen'}
          </Button>
        </div>
      )}
    </Card>
  )
}

function Composer({ text, setText, busy, onSend }: { text: string; setText: (t: string) => void; busy: boolean; onSend: (t: string) => void }) {
  const [listening, setListening] = useState(false)
  const rec = useRef<Recognition | null>(null)
  const base = useRef('')
  const submit = (e?: FormEvent) => { e?.preventDefault(); rec.current?.stop(); onSend(text) }
  const toggleMic = () => {
    if (!SpeechRecognition) return
    if (listening) { rec.current?.stop(); return }
    const r = new SpeechRecognition()
    r.lang = 'de-DE'
    r.interimResults = true
    r.continuous = true
    base.current = text ? text.trimEnd() + ' ' : ''
    r.onresult = (e) => {
      let t = ''
      for (let i = 0; i < e.results.length; i++) t += e.results[i][0].transcript
      setText(base.current + t)
    }
    r.onend = () => setListening(false)
    r.onerror = () => setListening(false)
    rec.current = r
    r.start()
    setListening(true)
  }
  return (
    <form onSubmit={submit} className="flex items-end gap-2 border-t border-line bg-card px-8 py-4">
      <textarea aria-label="Nachricht an den Assistenten" rows={2} value={text} onChange={(e) => setText(e.target.value)}
        onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit() } }}
        placeholder={SpeechRecognition ? 'Schreiben oder auf das Mikrofon tippen und sprechen …' : 'Schreiben oder mit Wispr Flow diktieren …'}
        className="min-h-[52px] flex-grow resize-none rounded-[12px] border-[1.5px] border-line bg-bg px-3.5 py-3 text-[15px] text-text outline-none focus:border-teal" />
      {SpeechRecognition && (
        <button type="button" onClick={toggleMic} aria-pressed={listening} aria-label={listening ? 'Diktat beenden' : 'Diktieren'}
          className={`flex h-[52px] w-[52px] shrink-0 items-center justify-center rounded-[12px] border-[1.5px] ${listening ? 'animate-pulse border-bad bg-bad-bg text-bad' : 'border-line text-text hover:bg-soft'}`}>
          <Icon name="mic" />
        </button>
      )}
      <Button type="submit" size="lg" icon="send" disabled={busy || !text.trim()} className="h-[52px]">Senden</Button>
    </form>
  )
}
