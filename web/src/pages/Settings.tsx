import { useState, type FormEvent, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, request, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon, type IconName } from '../components/Icon'
import { Button, Card, CardTitle, Chip, Field, inputClass } from '../components/ui'
import { fmtDate } from '../lib/format'
import { errorText, parseNumber } from '../lib/money'
import { useApp, type Account } from '../lib/state'

type Settings = Schemas['UserSettings']
type Session = Schemas['Session']

const currencies = ['EUR', 'CHF', 'USD', 'GBP', 'PLN', 'CZK', 'SEK', 'DKK', 'NOK', 'HUF']

function zones(): string[] {
  try {
    return (Intl as unknown as { supportedValuesOf(k: string): string[] }).supportedValuesOf('timeZone')
  } catch {
    return ['Europe/Berlin', 'Europe/Vienna', 'Europe/Zurich', 'Europe/London', 'UTC']
  }
}

function Section({ icon, title, sub, children }: { icon: IconName; title: string; sub?: string; children: ReactNode }) {
  return (
    <Card className="flex flex-col gap-4 p-6">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] bg-info-bg text-link"><Icon name={icon} /></div>
        <div className="flex flex-col gap-0.5">
          <CardTitle>{title}</CardTitle>
          {sub && <div className="text-[13px] text-muted">{sub}</div>}
        </div>
      </div>
      {children}
    </Card>
  )
}

function Saved({ ok, error }: { ok: boolean; error?: string }) {
  if (error) return <div role="alert" className="text-sm font-semibold text-bad">{error}</div>
  if (ok) return <div role="status" className="flex items-center gap-1.5 text-sm font-semibold text-ok"><Icon name="check" size={16} />Gespeichert</div>
  return null
}

export function SettingsPage() {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.get<Account>('/me') })
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => api.get<Settings>('/me/settings') })
  return (
    <>
      <Header title="Einstellungen" sub="Konto, Darstellung, Vorgaben und angemeldete Geräte" />
      <main className="flex min-h-0 flex-grow flex-col overflow-y-auto px-8 py-6">
        <div className="grid max-w-[1100px] grid-cols-1 gap-5 xl:grid-cols-2">
          {me.data && <ProfileSection me={me.data} />}
          <PasswordSection />
          <AppearanceSection />
          {settings.data && <DefaultsSection settings={settings.data} />}
          {settings.data && <ThresholdsSection settings={settings.data} />}
          <SessionsSection />
          <TokensSection />
        </div>
      </main>
    </>
  )
}

function ProfileSection({ me }: { me: Account }) {
  const qc = useQueryClient()
  const [name, setName] = useState(me.display_name)
  const save = useMutation({
    mutationFn: () => request<Account>('PATCH', '/me', { display_name: name.trim() }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }),
  })
  return (
    <Section icon="user" title="Profil" sub="So erscheinst du bei geteilten Fahrzeugen">
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); save.mutate() }}>
        <Field label="Anzeigename" htmlFor="s-name"><input id="s-name" required maxLength={100} className={inputClass} value={name} onChange={(e) => setName(e.target.value)} /></Field>
        <Field label="E-Mail" htmlFor="s-mail" hint="Die Anmeldeadresse ändert ein Administrator.">
          <input id="s-mail" readOnly className={`${inputClass} bg-soft text-muted`} value={me.email} />
        </Field>
        <div className="flex items-center justify-between gap-3">
          <Saved ok={save.isSuccess} error={save.error ? errorText(save.error) : undefined} />
          <Button type="submit" disabled={save.isPending || !name.trim() || name.trim() === me.display_name}>Speichern</Button>
        </div>
      </form>
    </Section>
  )
}

function PasswordSection() {
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  const qc = useQueryClient()
  const save = useMutation({
    mutationFn: () => api.post('/me/password', { current_password: cur, new_password: next }),
    onSuccess: () => { setCur(''); setNext(''); setRepeat(''); qc.invalidateQueries({ queryKey: ['sessions'] }) },
  })
  const mismatch = repeat !== '' && next !== repeat
  return (
    <Section icon="lock" title="Passwort ändern" sub="Danach werden alle anderen Geräte abgemeldet – auch die App">
      <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); save.mutate() }}>
        <Field label="Bisheriges Passwort" htmlFor="p-cur"><input id="p-cur" type="password" autoComplete="current-password" required className={inputClass} value={cur} onChange={(e) => setCur(e.target.value)} /></Field>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Neues Passwort" htmlFor="p-new" hint="Mindestens 12 Zeichen">
            <input id="p-new" type="password" autoComplete="new-password" required minLength={12} className={inputClass} value={next} onChange={(e) => setNext(e.target.value)} />
          </Field>
          <Field label="Wiederholen" htmlFor="p-rep" error={mismatch ? 'Stimmt nicht überein.' : undefined}>
            <input id="p-rep" type="password" autoComplete="new-password" required className={inputClass} value={repeat} onChange={(e) => setRepeat(e.target.value)} />
          </Field>
        </div>
        <div className="flex items-center justify-between gap-3">
          <Saved ok={save.isSuccess} error={save.error ? errorText(save.error) : undefined} />
          <Button type="submit" disabled={save.isPending || !cur || next.length < 12 || next !== repeat}>Passwort ändern</Button>
        </div>
      </form>
    </Section>
  )
}

function Choice<T extends string>({ value, options, onChange, label }: { value: T; options: [T, string][]; onChange: (v: T) => void; label: string }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-2">
      {options.map(([k, l]) => (
        <button key={k} type="button" role="radio" aria-checked={value === k} onClick={() => onChange(k)}
          className={`h-10 rounded-[10px] border px-4 text-sm font-semibold ${value === k ? 'border-teal bg-info-bg text-link' : 'border-line hover:bg-soft'}`}>
          {l}
        </button>
      ))}
    </div>
  )
}

function AppearanceSection() {
  const { theme, setTheme } = useApp()
  return (
    <Section icon="moon" title="Darstellung" sub="Gilt für diesen Browser">
      <Choice label="Farbschema" value={theme} onChange={setTheme} options={[['system', 'Wie System'], ['light', 'Hell'], ['dark', 'Dunkel']]} />
    </Section>
  )
}

function usePatchSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: Settings) => request<Settings>('PATCH', '/me/settings', patch).then((r) => r.data),
    onSuccess: (s) => { qc.setQueryData(['settings'], s); qc.invalidateQueries({ queryKey: ['maintenance'] }) },
  })
}

function DefaultsSection({ settings }: { settings: Settings }) {
  const save = usePatchSettings()
  const [tz, setTz] = useState(settings.time_zone ?? 'Europe/Berlin')
  const [cur, setCur] = useState(settings.default_currency ?? 'EUR')
  const [dist, setDist] = useState<'km' | 'mi'>(settings.display_units?.distance ?? 'km')
  const [vol, setVol] = useState<'l' | 'gal_us' | 'gal_imp'>(settings.display_units?.volume ?? 'l')
  const [exifShow, setExifShow] = useState(!!settings.show_exif_location)
  const [exifStore, setExifStore] = useState(!!settings.store_exif_location)
  const list = zones()
  return (
    <Section icon="gear" title="Vorgaben" sub="Zeitzone, Währung und Einheiten für neue Einträge und die Anzeige">
      <form className="flex flex-col gap-4" onSubmit={(e) => {
        e.preventDefault()
        save.mutate({ time_zone: tz, default_currency: cur, display_units: { distance: dist, volume: vol }, show_exif_location: exifShow, store_exif_location: exifStore })
      }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Zeitzone" htmlFor="d-tz">
            <select id="d-tz" className={inputClass} value={tz} onChange={(e) => setTz(e.target.value)}>
              {!list.includes(tz) && <option value={tz}>{tz}</option>}
              {list.map((z) => <option key={z} value={z}>{z.replaceAll('_', ' ')}</option>)}
            </select>
          </Field>
          <Field label="Währung" htmlFor="d-cur">
            <select id="d-cur" className={inputClass} value={cur} onChange={(e) => setCur(e.target.value)}>
              {[...new Set([cur, ...currencies])].map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </Field>
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-semibold">Strecke</span>
          <Choice label="Strecke" value={dist} onChange={setDist} options={[['km', 'Kilometer'], ['mi', 'Meilen']]} />
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-semibold">Volumen</span>
          <Choice label="Volumen" value={vol} onChange={setVol} options={[['l', 'Liter'], ['gal_us', 'Gallonen (US)'], ['gal_imp', 'Gallonen (UK)']]} />
        </div>
        <div className="flex flex-col gap-2 text-sm">
          <span className="font-semibold">Fotos</span>
          <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={exifStore} onChange={(e) => setExifStore(e.target.checked)} />Aufnahmeort aus Fotos speichern</label>
          <label className="flex items-center gap-2"><input type="checkbox" className="h-4 w-4 accent-teal" checked={exifShow} onChange={(e) => setExifShow(e.target.checked)} />Aufnahmeort anzeigen</label>
          <span className="text-xs text-muted">Vorschaubilder enthalten nie einen Standort; das Original sehen nur Bearbeiter.</span>
        </div>
        <div className="flex items-center justify-between gap-3">
          <Saved ok={save.isSuccess} error={save.error ? errorText(save.error) : undefined} />
          <Button type="submit" disabled={save.isPending}>Speichern</Button>
        </div>
      </form>
    </Section>
  )
}

function ThresholdsSection({ settings }: { settings: Settings }) {
  const save = usePatchSettings()
  const t = settings.maintenance_thresholds
  const [up, setUp] = useState(String(t?.upcoming_days ?? 30))
  const [due, setDue] = useState(String(t?.due_days ?? 7))
  const [upKm, setUpKm] = useState(String(t?.upcoming_distance?.value ?? 1500))
  const [dueKm, setDueKm] = useState(String(t?.due_distance?.value ?? 500))
  const n = (s: string) => { const v = parseNumber(s); return v === null || v < 0 ? null : Math.round(v) }
  const valid = [up, due, upKm, dueKm].every((s) => n(s) !== null)
  return (
    <Section icon="wrench" title="Wartung" sub="Ab wann eine Wartung als „demnächst“ bzw. „fällig“ gilt – für alle deine Fahrzeuge">
      <form className="flex flex-col gap-4" onSubmit={(e) => {
        e.preventDefault()
        save.mutate({ maintenance_thresholds: { upcoming_days: n(up), due_days: n(due), upcoming_distance: { value: n(upKm)!, unit: 'km' }, due_distance: { value: n(dueKm)!, unit: 'km' } } })
      }}>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Demnächst ab … Tage vorher" htmlFor="t-up"><input id="t-up" inputMode="numeric" className={inputClass} value={up} onChange={(e) => setUp(e.target.value)} /></Field>
          <Field label="oder … km vorher" htmlFor="t-upkm"><input id="t-upkm" inputMode="numeric" className={inputClass} value={upKm} onChange={(e) => setUpKm(e.target.value)} /></Field>
          <Field label="Fällig ab … Tage vorher" htmlFor="t-due"><input id="t-due" inputMode="numeric" className={inputClass} value={due} onChange={(e) => setDue(e.target.value)} /></Field>
          <Field label="oder … km vorher" htmlFor="t-duekm"><input id="t-duekm" inputMode="numeric" className={inputClass} value={dueKm} onChange={(e) => setDueKm(e.target.value)} /></Field>
        </div>
        <div className="text-xs text-muted">Eine Wartung mit eigenen Schwellen behält diese. Für Fahrzeuge anderer Halter gelten deren Vorgaben.</div>
        <div className="flex items-center justify-between gap-3">
          <Saved ok={save.isSuccess} error={save.error ? errorText(save.error) : undefined} />
          <Button type="submit" disabled={save.isPending || !valid}>Speichern</Button>
        </div>
      </form>
    </Section>
  )
}

function deviceLabel(s: Session) {
  if (s.client_kind === 'android') return 'Android-App'
  const ua = s.user_agent ?? ''
  const browser = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : 'Browser'
  const os = /Windows/.test(ua) ? 'Windows' : /Mac OS X/.test(ua) ? 'macOS' : /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Linux/.test(ua) ? 'Linux' : ''
  return os ? `${browser} auf ${os}` : browser
}

function SessionsSection() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const sessions = useQuery({ queryKey: ['sessions'], queryFn: () => api.get<{ items: Session[] }>('/me/sessions'), refetchInterval: 60_000 })
  const [error, setError] = useState<string>()
  const revoke = useMutation({
    mutationFn: (s: Session) => request('DELETE', `/me/sessions/${s.id}`).then(() => s),
    onSuccess: (s) => {
      if (s.current) { qc.clear(); navigate('/login'); return }
      qc.invalidateQueries({ queryKey: ['sessions'] })
    },
    onError: (e) => setError(errorText(e)),
  })
  const items = sessions.data?.items ?? []
  return (
    <Section icon="key" title="Angemeldete Geräte" sub="Die App bleibt bis zu einem Jahr angemeldet, solange sie mindestens alle 90 Tage genutzt wird">
      <div className="flex flex-col">
        {items.map((s, i) => (
          <div key={s.id} className={`flex items-center gap-3 py-3 ${i ? 'border-t border-line' : ''}`}>
            <span className="text-muted"><Icon name={s.client_kind === 'android' ? 'gauge' : 'home'} /></span>
            <div className="flex min-w-0 flex-grow flex-col">
              <div className="flex flex-wrap items-center gap-2 text-sm font-semibold">{deviceLabel(s)}{s.current && <Chip tone="ok">Dieses Gerät</Chip>}</div>
              <div className="text-xs text-muted">Zuletzt aktiv {s.last_seen_at ? fmtDate(s.last_seen_at) : '–'} · angemeldet {fmtDate(s.created_at)}</div>
            </div>
            <Button variant="outline" disabled={revoke.isPending} onClick={() => revoke.mutate(s)}>{s.current ? 'Abmelden' : 'Beenden'}</Button>
          </div>
        ))}
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
      </div>
    </Section>
  )
}

type ApiToken = Schemas['ApiToken']

/** KI-Zugang: persönliche API-Tokens für den MCP-Server (ADR-032). */
function TokensSection() {
  const qc = useQueryClient()
  const { vehicles } = useApp()
  const tokens = useQuery({ queryKey: ['api-tokens'], queryFn: () => api.get<{ items: ApiToken[] }>('/me/api-tokens') })
  const [name, setName] = useState('Claude Desktop')
  const [write, setWrite] = useState(true)
  const [days, setDays] = useState(90)
  const [only, setOnly] = useState<string>('')
  const [created, setCreated] = useState<ApiToken | null>(null)
  const [copied, setCopied] = useState(false)
  const create = useMutation({
    mutationFn: () => api.post<ApiToken>('/me/api-tokens', {
      name: name.trim(), scopes: write ? ['vehicles:read', 'entries:write'] : ['vehicles:read'],
      vehicle_ids: only ? [only] : null, expires_at: new Date(Date.now() + days * 86400_000).toISOString(),
    }),
    onSuccess: (t) => { setCreated(t); setCopied(false); qc.invalidateQueries({ queryKey: ['api-tokens'] }) },
  })
  const revoke = useMutation({
    mutationFn: (id: string) => request('DELETE', `/me/api-tokens/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-tokens'] }),
  })
  const url = `${location.origin}/api/v1/mcp`
  const config = created ? JSON.stringify({ mcpServers: { vectra: { type: 'http', url, headers: { Authorization: `Bearer ${created.token}` } } } }, null, 2) : ''
  const cli = created ? `claude mcp add --transport http vectra ${url} --header "Authorization: Bearer ${created.token}"` : ''
  return (
    <Section icon="sparkles" title="KI-Zugang (MCP)" sub="Persönliche Tokens für Claude Desktop, Claude Code und andere MCP-Clients sowie für die REST-API (Skripte, Home Assistant; docs/api.md)">
      <div className="flex flex-col">
        {(tokens.data?.items ?? []).map((t, i) => (
          <div key={t.id} className={`flex items-center gap-3 py-3 ${i ? 'border-t border-line' : ''}`}>
            <span className="text-muted"><Icon name="key" /></span>
            <div className="flex min-w-0 flex-grow flex-col">
              <div className="flex flex-wrap items-center gap-2 text-sm font-semibold">{t.name}
                <Chip tone={t.scopes.includes('entries:write') ? 'warn' : 'info'}>{t.scopes.includes('entries:write') ? 'Lesen + Schreiben' : 'Nur lesen'}</Chip>
              </div>
              <div className="text-xs text-muted">gültig bis {fmtDate(t.expires_at, true)} · zuletzt genutzt {t.last_used_at ? fmtDate(t.last_used_at) : 'nie'}</div>
            </div>
            <Button variant="outline" disabled={revoke.isPending} onClick={() => revoke.mutate(t.id!)}>Widerrufen</Button>
          </div>
        ))}
      </div>
      {created ? (
        <div className="flex flex-col gap-3 rounded-[12px] bg-ok-bg p-4 text-[13px]">
          <div className="font-semibold text-ok">Token erstellt – es wird nur jetzt angezeigt.</div>
          <code className="block break-all rounded-[8px] bg-card px-3 py-2 text-text">{created.token}</code>
          <div className="text-text">Claude Code:</div>
          <code className="block break-all rounded-[8px] bg-card px-3 py-2 text-text">{cli}</code>
          <div className="text-text">MCP-Konfiguration (Clients mit HTTP-Transport):</div>
          <pre className="m-0 overflow-x-auto rounded-[8px] bg-card px-3 py-2 text-text">{config}</pre>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { void navigator.clipboard?.writeText(created.token ?? ''); setCopied(true) }}>{copied ? 'Kopiert' : 'Token kopieren'}</Button>
            <Button variant="ghost" onClick={() => setCreated(null)}>Fertig</Button>
          </div>
        </div>
      ) : (
        <form className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); create.mutate() }}>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Name" htmlFor="t-name"><input id="t-name" required maxLength={100} className={inputClass} value={name} onChange={(e) => setName(e.target.value)} /></Field>
            <Field label="Gültig" htmlFor="t-days">
              <select id="t-days" className={inputClass} value={days} onChange={(e) => setDays(Number(e.target.value))}>
                <option value={30}>30 Tage</option><option value={90}>90 Tage</option><option value={365}>1 Jahr</option>
              </select>
            </Field>
          </div>
          <Field label="Fahrzeuge" htmlFor="t-veh">
            <select id="t-veh" className={inputClass} value={only} onChange={(e) => setOnly(e.target.value)}>
              <option value="">Alle meine Fahrzeuge</option>
              {vehicles.map((v) => <option key={v.id} value={v.id}>Nur {v.display_name}</option>)}
            </select>
          </Field>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="h-4 w-4 accent-teal" checked={write} onChange={(e) => setWrite(e.target.checked)} />
            Einträge anlegen erlauben (Kilometerstand, Fahrten, Kosten …); der Client fragt vor jedem Aufruf nach</label>
          <div className="flex items-center justify-between gap-3">
            <Saved ok={false} error={create.error ? errorText(create.error) : undefined} />
            <Button type="submit" icon="plus" disabled={create.isPending || !name.trim()}>Token erstellen</Button>
          </div>
        </form>
      )}
    </Section>
  )
}
