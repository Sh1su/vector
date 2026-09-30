import type { ReactNode } from 'react'

export function Header({ title, sub, actions }: { title: string; sub?: string; actions?: ReactNode }) {
  return (
    <header className="flex min-h-[76px] shrink-0 items-center gap-3.5 border-b border-line bg-card px-8 py-3">
      <div className="flex min-w-0 flex-grow flex-col gap-0.5">
        <h1 className="m-0 font-display text-[22px] leading-tight font-semibold">{title}</h1>
        {sub && <div className="truncate text-[13px] text-muted">{sub}</div>}
      </div>
      {actions}
    </header>
  )
}
