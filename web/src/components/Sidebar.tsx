import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { Icon, type IconName } from './Icon'
import { api } from '../api/client'
import { useApp, vehicleSubtitle, type Account } from '../lib/state'

interface NavItem { to: string; label: string; icon: IconName }

const groups: { title: string; items: NavItem[] }[] = [
  { title: 'Alltag', items: [
    { to: '/', label: 'Übersicht', icon: 'home' },
    { to: '/fahrten', label: 'Fahrten', icon: 'route' },
    { to: '/wartung', label: 'Wartung', icon: 'wrench' },
    { to: '/service', label: 'Servicehistorie', icon: 'receipt' },
    { to: '/kosten', label: 'Kosten', icon: 'euro' },
  ] },
  { title: 'Fahrzeug', items: [
    { to: '/fahrzeuge', label: 'Fahrzeuge', icon: 'garage' },
    { to: '/kilometer', label: 'Kilometerstand', icon: 'gauge' },
    { to: '/kraftstoff', label: 'Kraftstoff', icon: 'fuel' },
    { to: '/oel', label: 'Öl', icon: 'oil' },
    { to: '/dokumente', label: 'Dokumente', icon: 'doc' },
  ] },
  { title: 'Hilfe & Konto', items: [
    { to: '/assistent', label: 'Assistent', icon: 'sparkles' },
    { to: '/einstellungen', label: 'Einstellungen', icon: 'gear' },
  ] },
]

export function Sidebar({ me }: { me: Account }) {
  const { vehicles, vehicle, setVehicleId, theme, setTheme } = useApp()
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const qc = useQueryClient()

  async function logout() {
    try { await api.post('/auth/logout') } finally {
      qc.clear()
      navigate('/login')
    }
  }
  const dark = theme === 'dark' || (theme === 'system' && document.documentElement.classList.contains('dark'))

  return (
    <nav aria-label="Hauptnavigation" className="flex h-full w-[248px] shrink-0 flex-col gap-4 border-r border-sidebar-edge bg-sidebar px-3.5 py-5">
      <NavLink to="/" aria-label="Vectra Startseite" className="flex items-center gap-2.5 px-1.5">
        <img src="/brand/vectra-bildmarke-negativ.png" alt="" className="h-[38px] w-[51px] object-contain" />
        <img src="/brand/vectra-schriftzug-negativ.png" alt="Vectra" className="h-7 w-[118px] object-contain" />
      </NavLink>

      <div className="relative">
        <button type="button" onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-haspopup="listbox"
          className="flex w-full items-center gap-2.5 rounded-[12px] bg-white/8 px-3 py-2.5 text-left">
          <span className="text-teal-soft"><Icon name="car" /></span>
          <span className="flex min-w-0 flex-grow flex-col">
            <span className="truncate text-sm font-semibold text-paper">{vehicle?.display_name ?? 'Kein Fahrzeug'}</span>
            <span className="truncate text-xs text-slate-300">{vehicle ? vehicleSubtitle(vehicle) : 'Fahrzeug anlegen'}</span>
          </span>
          <span className="text-slate-300"><Icon name="down" size={16} /></span>
        </button>
        {open && (
          <ul role="listbox" aria-label="Fahrzeug wählen" className="absolute top-full right-0 left-0 z-20 mt-1 overflow-hidden rounded-[12px] border border-line bg-card py-1 shadow-lg">
            {vehicles.map((v) => (
              <li key={v.id} role="option" aria-selected={v.id === vehicle?.id}>
                <button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-soft"
                  onClick={() => { setVehicleId(v.id); setOpen(false) }}>
                  <Icon name={v.id === vehicle?.id ? 'check' : 'car'} size={16} className="text-link" />
                  <span className="truncate">{v.display_name}</span>
                </button>
              </li>
            ))}
            <li><NavLink to="/fahrzeuge" onClick={() => setOpen(false)} className="flex items-center gap-2 px-3 py-2 text-sm font-semibold text-link hover:bg-soft"><Icon name="plus" size={16} />Fahrzeuge verwalten</NavLink></li>
          </ul>
        )}
      </div>

      {groups.map((g) => (
        <div key={g.title} className="flex flex-col gap-0.5">
          <div className="px-2.5 pb-1 text-[11px] font-bold tracking-[0.08em] text-slate-400 uppercase">{g.title}</div>
          {g.items.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.to === '/'}
              className={({ isActive }) => `flex h-[38px] items-center gap-2.5 rounded-[10px] px-2.5 text-sm ${isActive ? 'bg-teal/22 font-semibold text-paper' : 'font-medium text-slate-300 hover:bg-white/5'}`}>
              {({ isActive }) => (<><span className={isActive ? 'text-teal-soft' : 'text-slate-400'}><Icon name={n.icon} /></span>{n.label}</>)}
            </NavLink>
          ))}
        </div>
      ))}

      <div className="flex-grow" />
      <button type="button" onClick={() => setTheme(dark ? 'light' : 'dark')}
        className="flex h-[38px] items-center gap-2.5 rounded-[10px] px-2.5 text-sm font-medium text-slate-300 hover:bg-white/5">
        <span className="text-slate-400"><Icon name={dark ? 'sun' : 'moon'} /></span>{dark ? 'Heller Modus' : 'Dunkler Modus'}
      </button>
      <div className="flex items-center gap-2.5 border-t border-white/12 px-1 pt-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-teal/25 text-paper"><Icon name="user" /></div>
        <div className="flex min-w-0 flex-grow flex-col">
          <div className="truncate text-[13px] font-semibold text-paper">{me.display_name}</div>
          <div className="truncate text-xs text-slate-300">{me.email}</div>
        </div>
        <button type="button" onClick={logout} aria-label="Abmelden" className="flex h-9 w-9 items-center justify-center rounded-[10px] text-slate-300 hover:bg-white/5"><Icon name="logout" /></button>
      </div>
    </nav>
  )
}
