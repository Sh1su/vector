// Bestätigungsdialog für Plausibilitätsbefunde (ADR-010): nicht bestätigbare
// Befunde müssen korrigiert werden, bestätigbare brauchen eine Begründung.
import { useState } from 'react'
import type { Anomaly } from '../api/client'
import { Icon } from './Icon'
import { Button, inputClass } from './ui'

export function PlausibilityAlert({ anomalies, onConfirm, onEdit, busy }:
  { anomalies: Anomaly[]; onConfirm: (codes: string[], reason: string) => void; onEdit: () => void; busy?: boolean }) {
  const [reason, setReason] = useState('')
  const blocking = anomalies.filter((a) => !a.confirmable)
  return (
    <div role="alert" className="flex gap-3 rounded-[16px] border-[1.5px] border-amber bg-warn-bg p-4">
      <span className="text-warn"><Icon name="alert" size={22} /></span>
      <div className="flex flex-grow flex-col gap-3">
        <ul className="m-0 flex list-none flex-col gap-1 p-0 text-sm leading-relaxed">
          {anomalies.map((a) => <li key={a.code}><b>{label(a.code)}:</b> {a.message}</li>)}
        </ul>
        {blocking.length > 0 ? (
          <div className="text-sm">Diese Eingabe kann nicht gespeichert werden. Bitte korrigieren.</div>
        ) : (
          <>
            <label htmlFor="anomaly-reason" className="text-sm font-semibold">Begründung, falls der Wert trotzdem stimmt</label>
            <input id="anomaly-reason" className={inputClass + ' h-11'} value={reason} onChange={(e) => setReason(e.target.value)}
              placeholder="z. B. Vorheriger Wert war ein Tippfehler" />
          </>
        )}
        <div className="flex flex-wrap gap-3">
          <Button type="button" variant="outline" icon="edit" onClick={onEdit}>Korrigieren</Button>
          {blocking.length === 0 && (
            <Button type="button" variant="navy" disabled={!reason.trim() || busy} onClick={() => onConfirm(anomalies.map((a) => a.code), reason.trim())}>
              Trotzdem speichern
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

function label(code: string) {
  return ({ P1: 'Rückläufig', P2: 'Größer als Folgewert', P3: 'Unrealistischer Sprung', P4: 'Zukunft', VIN_NONSTANDARD: 'FIN', VIN_DUPLICATE: 'FIN doppelt' } as Record<string, string>)[code] ?? code
}
