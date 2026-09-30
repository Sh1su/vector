import * as D from '@radix-ui/react-dialog'
import type { ReactNode } from 'react'
import { Icon } from './Icon'

export function Dialog({ open, onOpenChange, title, children }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; children: ReactNode }) {
  return (
    <D.Root open={open} onOpenChange={onOpenChange}>
      <D.Portal>
        <D.Overlay className="fixed inset-0 z-40 bg-ink/50" />
        <D.Content aria-describedby={undefined} className="fixed top-1/2 left-1/2 z-50 max-h-[90vh] w-[min(560px,calc(100vw-32px))] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-[16px] border border-line bg-card p-6 text-text shadow-2xl">
          <div className="mb-4 flex items-center justify-between gap-3">
            <D.Title className="m-0 font-display text-xl font-semibold">{title}</D.Title>
            <D.Close aria-label="Schließen" className="flex h-9 w-9 items-center justify-center rounded-[10px] hover:bg-soft"><Icon name="x" /></D.Close>
          </div>
          {children}
        </D.Content>
      </D.Portal>
    </D.Root>
  )
}
