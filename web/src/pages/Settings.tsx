import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon, type IconName } from '../components/Icon'
import { Button, Card, Chip, Field, inputClass } from '../components/ui'
import { fmtDate, num } from '../lib/format'
import { problemText } from '../lib/mutation'
import { useApp, type Account } from '../lib/state'
import { ConsentCard, useAssistantStatus } from './Assistant'

type UserSettings = Schemas['UserSettings']
type Install = Schemas['InstallationSettings']

function Section({ icon, title, children, sub }: { icon: IconName; title: string; sub?: string; children: ReactNode }) {
  return (
    <Card className="flex flex-col gap-4 p-5">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] bg-info-bg text-link"><Icon name={icon} /></div>
        <div className="flex flex-col gap-0.5"><h2 className="m-0 font-display text-base font-semibold">{title}</h2>{sub && <div className="text-[13px] text-muted">{sub}</div>}</div>
      </div>
      {children}
    </Card>
  )
}

function useSaver() {
  const [state, setState] = useState<{ busy: boolean; msg?: string; error?: string }>({ busy: false })
  return {
    ...state,
    run: async (fn: () => Promise<unknown>, msg = 'Gespeichert.') => {
      setState({ busy: true })
      try { await fn(); setState({ busy: false, msg }) } catch (e) { setState({ busy: false, error: problemText(e) }) }
    },
  }
}

function Feedback({ s }: { s: { msg?: string; error?: string } }) {
  if (s.error) return <div role="alert" className="text-sm font-semibold text-bad">{s.error}</div>
  if (s.msg) return <div role="status" className="text-sm font-semibold text-ok">{s.msg}</div>
  return null
}

const select = (id: string, value: string, onChange: (v: string) => void, options: [string, string][]) => (
  <select id={id} className={inputClass} value={value} onChange={(e) => onChange(e.target.value)}>{options.map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select>
)

export function SettingsPage() {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.get<Account>('/me') }).data
  return (
    <>
      <Header title="Einstellungen" sub={me ? `${me.display_name} · ${me.email}` : undefined} />
      <main className="grid min-h-0 flex-grow grid-cols-1 items-start gap-5 overflow-y-auto px-8 py-6 xl:grid-cols-2">
        <div className="flex flex-col gap-5">
          <ProfileSection me={me} />
          <UnitsSection />
          <AssistantSection />
        </div>
        <div className="flex flex-col gap-5">
          <PasswordSection />
          <SessionsSection />
          {me?.is_admin && <InstallationSection />}
        </div>
      </main>
    </>
  )
}

function ProfileSection({ me }: { me?: Account }) {
  const qc = useQueryClient()
  const { theme, setTheme } = useApp()
  const [name, setName] = useState('')
  useEffect(() => { if (me) setName(me.display_name) }, [me])
  const s = useSaver()
  return (
    <Section icon="user" title="Profil">
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); s.run(async () => { await api.patch('/me', { display_name: name }, '*'); qc.invalidateQueries({ queryKey: ['me'] }) }) }}>
        <Field label="Anzeigename" htmlFor="p-name"><input id="p-name" className={inputClass} value={name} onChange={(e) => setName(e.target.value)} /></Field>
        <Field label="Darstellung" htmlFor="p-theme">{select('p-theme', theme, (v) => setTheme(v as typeof theme), [['system', 'wie das System'], ['light', 'hell'], ['dark', 'dunkel']])}</Field>
        <Feedback s={s} />
        <Button type="submit" disabled={s.busy || !name.trim()} className="self-start">Speichern</Button>
      </form>
    </Section>
  )
}

function UnitsSection() {
  const qc = useQueryClient()
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => api.get<UserSettings>('/me/settings') })
  const [f, setF] = useState<Record<string, string>>({})
  useEffect(() => {
    const st = settings.data
    if (!st) return
    const du = st.display_units ?? {}
    const th = st.maintenance_thresholds ?? {}
    setF({ time_zone: st.time_zone ?? 'Europe/Berlin', currency: st.default_currency ?? 'EUR', distance: du.distance ?? 'km', volume: du.volume ?? 'l',
      consumption: du.consumption ?? 'l_per_100km', electric: du.electric_consumption ?? 'kwh_per_100km', oil: du.oil_volume ?? 'l',
      upDays: th.upcoming_days?.toString() ?? '', dueDays: th.due_days?.toString() ?? '',
      upKm: th.upcoming_distance ? String(th.upcoming_distance.value) : '', dueKm: th.due_distance ? String(th.due_distance.value) : '' })
  }, [settings.data])
  const s = useSaver()
  const set = (k: string) => (v: string) => setF({ ...f, [k]: v })
  function save(e: FormEvent) {
    e.preventDefault()
    const q = (v: string) => (v ? { value: num(v), unit: f.distance } : null)
    s.run(async () => {
      await api.patch('/me/settings', {
        time_zone: f.time_zone, default_currency: f.currency.toUpperCase(),
        display_units: { distance: f.distance, volume: f.volume, consumption: f.consumption, electric_consumption: f.electric, oil_volume: f.oil },
        maintenance_thresholds: { upcoming_days: f.upDays ? Number(f.upDays) : null, due_days: f.dueDays ? Number(f.dueDays) : null, upcoming_distance: q(f.upKm), due_distance: q(f.dueKm) },
      }, '*')
      qc.invalidateQueries()
    })
  }
  if (!f.distance) return <Section icon="gauge" title="Einheiten und Vorgaben"><div className="text-sm text-muted">Lade …</div></Section>
  return (
    <Section icon="gauge" title="Einheiten und Vorgaben" sub="Betrifft nur die Anzeige; gespeichert wird immer in SI-Einheiten mit Originaleingabe.">
      <form className="flex flex-col gap-4" onSubmit={save}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Strecke" htmlFor="u-dist">{select('u-dist', f.distance, set('distance'), [['km', 'Kilometer'], ['mi', 'Meilen']])}</Field>
          <Field label="Volumen" htmlFor="u-vol">{select('u-vol', f.volume, set('volume'), [['l', 'Liter'], ['gal_us', 'Gallonen (US)'], ['gal_imp', 'Gallonen (UK)']])}</Field>
          <Field label="Verbrauch" htmlFor="u-cons">{select('u-cons', f.consumption, set('consumption'), [['l_per_100km', 'l/100 km'], ['km_per_l', 'km/l'], ['mpg_us', 'mpg (US)'], ['mpg_uk', 'mpg (UK)']])}</Field>
          <Field label="Stromverbrauch" htmlFor="u-el">{select('u-el', f.electric, set('electric'), [['kwh_per_100km', 'kWh/100 km'], ['km_per_kwh', 'km/kWh'], ['mi_per_kwh', 'mi/kWh']])}</Field>
          <Field label="Ölmenge" htmlFor="u-oil">{select('u-oil', f.oil, set('oil'), [['ml', 'Milliliter'], ['l', 'Liter'], ['qt_us', 'Quart (US)'], ['qt_imp', 'Quart (UK)']])}</Field>
          <Field label="Zeitzone" htmlFor="u-tz"><input id="u-tz" className={inputClass} value={f.time_zone} onChange={(e) => set('time_zone')(e.target.value)} /></Field>
          <Field label="Währung" htmlFor="u-cur" hint="ISO 4217, z. B. EUR"><input id="u-cur" maxLength={3} className={inputClass + ' uppercase'} value={f.currency} onChange={(e) => set('currency')(e.target.value)} /></Field>
        </div>
        <div className="text-sm font-semibold">Wartung: ab wann „demnächst“ bzw. „fällig“</div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <Field label="demnächst (Tage)" htmlFor="u-ud"><input id="u-ud" inputMode="numeric" className={inputClass} value={f.upDays} onChange={(e) => set('upDays')(e.target.value)} placeholder="30" /></Field>
          <Field label="fällig (Tage)" htmlFor="u-dd"><input id="u-dd" inputMode="numeric" className={inputClass} value={f.dueDays} onChange={(e) => set('dueDays')(e.target.value)} placeholder="7" /></Field>
          <Field label={`demnächst (${f.distance})`} htmlFor="u-uk"><input id="u-uk" inputMode="decimal" className={inputClass} value={f.upKm} onChange={(e) => set('upKm')(e.target.value)} placeholder="1500" /></Field>
          <Field label={`fällig (${f.distance})`} htmlFor="u-dk"><input id="u-dk" inputMode="decimal" className={inputClass} value={f.dueKm} onChange={(e) => set('dueKm')(e.target.value)} placeholder="500" /></Field>
        </div>
        <div className="text-xs text-muted">Leer = Vorgabe der Installation. Eigene Schwellen einer einzelnen Wartung haben Vorrang.</div>
        <Feedback s={s} />
        <Button type="submit" disabled={s.busy} className="self-start">Speichern</Button>
      </form>
    </Section>
  )
}

function PasswordSection() {
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [again, setAgain] = useState('')
  const s = useSaver()
  const qc = useQueryClient()
  return (
    <Section icon="lock" title="Passwort ändern" sub="Andere angemeldete Geräte werden dabei abgemeldet.">
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => {
        e.preventDefault()
        s.run(async () => { await api.post('/me/password', { current_password: cur, new_password: next }); setCur(''); setNext(''); setAgain(''); qc.invalidateQueries({ queryKey: ['sessions'] }) }, 'Passwort geändert.')
      }}>
        <Field label="Aktuelles Passwort" htmlFor="pw-cur"><input id="pw-cur" type="password" autoComplete="current-password" required className={inputClass} value={cur} onChange={(e) => setCur(e.target.value)} /></Field>
        <Field label="Neues Passwort" htmlFor="pw-new" hint="mindestens 12 Zeichen"><input id="pw-new" type="password" autoComplete="new-password" required minLength={12} className={inputClass} value={next} onChange={(e) => setNext(e.target.value)} /></Field>
        <Field label="Neues Passwort wiederholen" htmlFor="pw-again" error={again && again !== next ? 'Stimmt nicht überein.' : undefined}>
          <input id="pw-again" type="password" autoComplete="new-password" required className={inputClass} value={again} onChange={(e) => setAgain(e.target.value)} /></Field>
        <Feedback s={s} />
        <Button type="submit" disabled={s.busy || next.length < 12 || next !== again} className="self-start">Passwort ändern</Button>
      </form>
    </Section>
  )
}

function SessionsSection() {
  const qc = useQueryClient()
  const sessions = useQuery({ queryKey: ['sessions'], queryFn: () => api.get<{ items: Schemas['Session'][] }>('/me/sessions') })
  async function revoke(id: string) {
    await api.del(`/me/sessions/${id}`, '*')
    qc.invalidateQueries({ queryKey: ['sessions'] })
  }
  return (
    <Section icon="key" title="Angemeldete Geräte">
      <div className="flex flex-col">
        {(sessions.data?.items ?? []).map((x) => (
          <div key={x.id} className="flex items-center gap-3 border-t border-line py-2.5 first:border-t-0">
            <div className="flex min-w-0 flex-grow flex-col gap-0.5">
              <div className="flex items-center gap-2 text-sm font-semibold">{x.client_kind === 'android' ? 'Android-App' : 'Browser'}{x.current && <Chip tone="ok">dieses Gerät</Chip>}</div>
              <div className="truncate text-xs text-muted">{x.user_agent || 'unbekannt'} · zuletzt {x.last_seen_at ? fmtDate(x.last_seen_at) : '–'}</div>
            </div>
            {!x.current && <Button variant="outline" onClick={() => revoke(x.id)}>Abmelden</Button>}
          </div>
        ))}
      </div>
    </Section>
  )
}

function AssistantSection() {
  const status = useAssistantStatus().data
  const qc = useQueryClient()
  if (!status?.enabled) return null
  return (
    <Section icon="sparkles" title="Assistent" sub={status.chat_provider ? `Anbieter: ${status.chat_provider.name}${status.chat_provider.external ? ' (extern)' : ' (lokal)'}` : undefined}>
      {status.user_enabled ? (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-sm">Eingeschaltet{status.consent_given_at ? ` seit ${fmtDate(status.consent_given_at, true)}` : ''}.</div>
          <Button variant="outline" onClick={async () => { await api.del('/assistant/consent', '*'); qc.invalidateQueries({ queryKey: ['assistant'] }) }}>Ausschalten</Button>
        </div>
      ) : <ConsentCard status={status} />}
    </Section>
  )
}

function InstallationSection() {
  const qc = useQueryClient()
  const inst = useQuery({ queryKey: ['installation'], queryFn: () => api.get<Install>('/admin/settings') })
  const status = useAssistantStatus().data
  const [f, setF] = useState<Record<string, string>>({})
  const [assistant, setAssistant] = useState(false)
  useEffect(() => {
    const i = inst.data
    if (!i) return
    const th = i.default_thresholds ?? {}
    setAssistant(!!i.assistant_enabled)
    setF({ upDays: th.upcoming_days?.toString() ?? '', dueDays: th.due_days?.toString() ?? '', upKm: th.upcoming_distance ? String(th.upcoming_distance.value) : '',
      dueKm: th.due_distance ? String(th.due_distance.value) : '', retention: i.retention_days?.toString() ?? '30' })
  }, [inst.data])
  const s = useSaver()
  const set = (k: string) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  function save(e: FormEvent) {
    e.preventDefault()
    const q = (v: string) => (v ? { value: num(v), unit: 'km' } : null)
    s.run(async () => {
      await api.patch('/admin/settings', { assistant_enabled: assistant, retention_days: Number(f.retention) || 30,
        default_thresholds: { upcoming_days: f.upDays ? Number(f.upDays) : null, due_days: f.dueDays ? Number(f.dueDays) : null, upcoming_distance: q(f.upKm), due_distance: q(f.dueKm) } }, '*')
      qc.invalidateQueries()
    })
  }
  return (
    <Section icon="gear" title="Installation" sub="Nur für Administratoren. Gilt für alle Konten.">
      {!inst.data ? <div className="text-sm text-muted">Lade …</div> : (
        <form className="flex flex-col gap-4" onSubmit={save}>
          <label className="flex items-start gap-2.5 text-sm">
            <input type="checkbox" className="mt-0.5 h-4 w-4 accent-teal" checked={assistant} disabled={!status?.chat_provider} onChange={(e) => setAssistant(e.target.checked)} />
            <span><b>KI-Assistent aktivieren</b><br /><span className="text-muted">{status?.chat_provider
              ? `Anbieter: ${status.chat_provider.name} (${status.chat_provider.external ? 'extern – jede Person muss zustimmen' : 'lokal'}).`
              : 'Kein Anbieter konfiguriert (VECTRA_ASSISTANT_PROVIDER, siehe deploy/README.md).'}</span></span>
          </label>
          <div className="text-sm font-semibold">Vorgaben für Wartungsschwellen</div>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <Field label="demnächst (Tage)" htmlFor="i-ud"><input id="i-ud" inputMode="numeric" className={inputClass} value={f.upDays ?? ''} onChange={set('upDays')} /></Field>
            <Field label="fällig (Tage)" htmlFor="i-dd"><input id="i-dd" inputMode="numeric" className={inputClass} value={f.dueDays ?? ''} onChange={set('dueDays')} /></Field>
            <Field label="demnächst (km)" htmlFor="i-uk"><input id="i-uk" inputMode="decimal" className={inputClass} value={f.upKm ?? ''} onChange={set('upKm')} /></Field>
            <Field label="fällig (km)" htmlFor="i-dk"><input id="i-dk" inputMode="decimal" className={inputClass} value={f.dueKm ?? ''} onChange={set('dueKm')} /></Field>
          </div>
          <Field label="Aufbewahrung (Tage)" htmlFor="i-ret"><input id="i-ret" inputMode="numeric" className={inputClass + ' max-w-[160px]'} value={f.retention ?? ''} onChange={set('retention')} /></Field>
          <div className="text-xs text-muted">Plausibilitätsgrenze Kilometerstand: {inst.data.odometer_v_max_kmh} km/h (Server-Konfiguration).</div>
          <Feedback s={s} />
          <Button type="submit" disabled={s.busy} className="self-start">Speichern</Button>
        </form>
      )}
    </Section>
  )
}
