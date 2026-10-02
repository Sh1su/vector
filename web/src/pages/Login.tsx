import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Schemas } from '../api/client'
import { Icon, type IconName } from '../components/Icon'
import { Button, Field, inputClass } from '../components/ui'

const points: { icon: IconName; title: string; text: string }[] = [
  { icon: 'wrench', title: 'Wartung im Griff', text: 'Intervalle nach km oder Zeit, Hinweise bevor etwas fällig wird.' },
  { icon: 'route', title: 'Fahrtenbuch mit einem Klick', text: 'Starten, beenden, fertig. Kategorien für Steuer und Abrechnung.' },
  { icon: 'sparkles', title: 'Assistent mit Quellen', text: 'Antworten aus Handbuch und Rechnungen, lokal ausgeführt.' },
]

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-full bg-bg text-text">
      <section className="hidden w-[680px] shrink-0 flex-col justify-between bg-navy px-16 py-14 lg:flex">
        <img src="/brand/vectra-logo-negativ.png" alt="Vectra – Intelligent Vehicle Management" className="h-[150px] w-[300px] object-contain object-left" />
        <div className="flex flex-col gap-7">
          <div className="max-w-[480px] font-display text-4xl leading-tight font-semibold text-paper">Intelligentes Fahrzeugmanagement auf einen Blick.</div>
          {points.map((p) => (
            <div key={p.title} className="flex items-start gap-3.5">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] bg-teal/22 text-teal-soft"><Icon name={p.icon} size={22} /></div>
              <div className="flex flex-col gap-0.5"><div className="text-base font-semibold text-paper">{p.title}</div><div className="text-sm leading-normal text-slate-300">{p.text}</div></div>
            </div>
          ))}
        </div>
        <div className="text-[13px] text-slate-400">Selbst gehostet · Deine Daten bleiben auf deinem Server.</div>
      </section>
      <section className="flex flex-grow items-center justify-center p-6">{children}</section>
    </div>
  )
}

function useSetupStatus() {
  return useQuery({ queryKey: ['setup-status'], queryFn: () => api.get<Schemas['SetupStatus']>('/auth/setup'), retry: false })
}

function errorText(e: unknown) {
  if (e instanceof ProblemError) return e.problem.errors?.[0]?.message || e.problem.detail || e.problem.title
  return 'Verbindung zum Server fehlgeschlagen.'
}

export function LoginPage() {
  const setup = useSetupStatus()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const qc = useQueryClient()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(undefined)
    try {
      await api.post('/auth/login', { email, password })
      await qc.invalidateQueries()
      navigate('/')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  if (setup.data?.setup_required) return <Navigate to="/einrichtung" replace />
  return (
    <AuthLayout>
      <form onSubmit={submit} className="flex w-full max-w-[400px] flex-col gap-4.5">
        <img src="/brand/vectra-logo-quer.png" alt="Vectra" className="h-12 w-[200px] object-contain object-left lg:hidden" />
        <div className="flex flex-col gap-1.5"><h1 className="m-0 font-display text-[28px] font-semibold">Anmelden</h1><div className="text-sm text-muted">Willkommen zurück bei Vectra.</div></div>
        <Field label="E-Mail" htmlFor="login-mail">
          <input id="login-mail" type="email" autoComplete="email" required placeholder="name@beispiel.de" className={inputClass} value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label="Passwort" htmlFor="login-pw" error={error}>
          <input id="login-pw" type="password" autoComplete="current-password" required placeholder="Passwort" className={inputClass} value={password} onChange={(e) => setPassword(e.target.value)} />
        </Field>
        <Button type="submit" size="lg" disabled={busy}>{busy ? 'Anmelden …' : 'Anmelden'}</Button>
        <div className="text-center text-[13px] leading-normal text-muted">Noch kein Konto? Frag deine Administration nach einer Einladung.</div>
      </form>
    </AuthLayout>
  )
}

export function SetupPage() {
  const setup = useSetupStatus()
  const [form, setForm] = useState({ setup_token: '', display_name: '', email: '', password: '' })
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const qc = useQueryClient()
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value })

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(undefined)
    try {
      const body: Record<string, string> = { display_name: form.display_name, email: form.email, password: form.password }
      if (setup.data?.token_required) body.setup_token = form.setup_token
      await api.post('/auth/setup', body)
      await api.post('/auth/login', { email: form.email, password: form.password })
      await qc.invalidateQueries()
      navigate('/')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  if (setup.data && !setup.data.setup_required) return <Navigate to="/login" replace />
  return (
    <AuthLayout>
      <form onSubmit={submit} className="flex w-full max-w-[400px] flex-col gap-4">
        <div className="flex flex-col gap-1.5"><h1 className="m-0 font-display text-[28px] font-semibold">Ersteinrichtung</h1>
          <div className="text-sm leading-normal text-muted">Willkommen! Lege das erste Administratorkonto an. Das geht nur einmal, solange noch kein Konto existiert.</div></div>
        {setup.data?.token_required && (
          <Field label="Setup-Token" htmlFor="setup-token" hint="steht in der .env als VECTRA_SETUP_TOKEN (vom Installationsskript erzeugt)">
            <input id="setup-token" required className={inputClass} value={form.setup_token} onChange={set('setup_token')} autoComplete="off" />
          </Field>
        )}
        <Field label="Name" htmlFor="setup-name"><input id="setup-name" required className={inputClass} value={form.display_name} onChange={set('display_name')} autoComplete="name" /></Field>
        <Field label="E-Mail" htmlFor="setup-mail"><input id="setup-mail" type="email" required className={inputClass} value={form.email} onChange={set('email')} autoComplete="email" /></Field>
        <Field label="Passwort" htmlFor="setup-pw" hint="Mindestens 12 Zeichen." error={error}>
          <input id="setup-pw" type="password" required minLength={12} className={inputClass} value={form.password} onChange={set('password')} autoComplete="new-password" />
        </Field>
        <Button type="submit" size="lg" disabled={busy}>Installation einrichten</Button>
        <div className="text-center text-[13px]"><Link to="/login" className="font-semibold text-link">Zur Anmeldung</Link></div>
      </form>
    </AuthLayout>
  )
}
