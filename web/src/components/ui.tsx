import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Icon, type IconName } from './Icon'

type Variant = 'primary' | 'navy' | 'outline' | 'ghost'

const variants: Record<Variant, string> = {
  primary: 'bg-teal text-ink hover:brightness-95',
  navy: 'bg-navy text-white hover:brightness-110',
  outline: 'border-[1.5px] border-line bg-card text-text hover:bg-soft',
  ghost: 'text-text hover:bg-soft',
}

export function Button({ variant = 'primary', icon, children, className = '', size = 'md', ...rest }:
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; icon?: IconName; size?: 'md' | 'lg' }) {
  const h = size === 'lg' ? 'h-12 px-5 text-[15px] font-display' : 'h-10 px-4 text-sm'
  return (
    <button {...rest} className={`inline-flex items-center justify-center gap-2 rounded-[12px] font-semibold whitespace-nowrap transition disabled:opacity-50 disabled:cursor-not-allowed ${h} ${variants[variant]} ${className}`}>
      {icon && <Icon name={icon} size={18} />}
      {children}
    </button>
  )
}

export function Card({ children, className = '', as: Tag = 'section' }: { children: ReactNode; className?: string; as?: 'section' | 'div' }) {
  return <Tag className={`rounded-[16px] border border-line bg-card ${className}`}>{children}</Tag>
}

export function CardTitle({ children, aside }: { children: ReactNode; aside?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <h2 className="m-0 font-display text-base font-semibold">{children}</h2>
      {aside}
    </div>
  )
}

type Tone = 'ok' | 'warn' | 'bad' | 'info' | 'neutral'
const tones: Record<Tone, string> = {
  ok: 'bg-ok-bg text-ok', warn: 'bg-warn-bg text-warn', bad: 'bg-bad-bg text-bad', info: 'bg-info-bg text-link', neutral: 'bg-soft text-muted',
}

export function Chip({ tone = 'neutral', icon, children }: { tone?: Tone; icon?: IconName; children: ReactNode }) {
  return (
    <span className={`inline-flex h-[26px] items-center gap-1.5 rounded-full px-2.5 text-xs font-semibold whitespace-nowrap ${tones[tone]}`}>
      {icon && <Icon name={icon} size={14} />}
      {children}
    </span>
  )
}

export function Field({ label, htmlFor, hint, error, children }: { label: string; htmlFor: string; hint?: string; error?: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={htmlFor} className="text-sm font-semibold">{label}</label>
      {children}
      {hint && !error && <div className="text-xs text-muted">{hint}</div>}
      {error && <div className="text-xs font-semibold text-bad" role="alert">{error}</div>}
    </div>
  )
}

export const inputClass = 'h-12 w-full rounded-[12px] border-[1.5px] border-line bg-card px-3.5 text-[15px] text-text outline-none focus:border-teal'

export function EmptyState({ icon, title, text, action }: { icon: IconName; title: string; text: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-[16px] border-[1.5px] border-dashed border-line p-10 text-center">
      <div className="flex h-13 w-13 items-center justify-center rounded-[16px] bg-info-bg text-link"><Icon name={icon} size={26} /></div>
      <div className="font-display text-lg font-semibold">{title}</div>
      <div className="max-w-md text-sm leading-relaxed text-muted">{text}</div>
      {action}
    </div>
  )
}

export function Kpi({ icon, label, value, note }: { icon: IconName; label: string; value: string; note: string }) {
  return (
    <div className="flex flex-col gap-1.5 rounded-[16px] border border-line bg-card p-4">
      <div className="flex items-center gap-2 text-[13px] font-medium text-muted"><span className="text-link"><Icon name={icon} /></span>{label}</div>
      <div className="tabular font-display text-[22px] font-semibold">{value}</div>
      <div className="text-xs text-muted">{note}</div>
    </div>
  )
}

export function Segmented<T extends string>({ value, options, onChange, label }: { value: T; options: [T, string][]; onChange: (v: T) => void; label: string }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-1 rounded-[12px] bg-soft p-1">
      {options.map(([k, l]) => (
        <button key={k} type="button" role="radio" aria-checked={k === value} onClick={() => onChange(k)}
          className={`h-9 rounded-[9px] px-3 text-sm font-semibold ${k === value ? 'bg-card text-text shadow-sm' : 'text-muted'}`}>{l}</button>
      ))}
    </div>
  )
}
