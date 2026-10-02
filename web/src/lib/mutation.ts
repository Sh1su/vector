// Mutation mit Plausibilitätsdialog (ADR-010): 422 mit Befunden → Bestätigen
// mit Begründung; andere Fehler als Text.
import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ProblemError, type Anomaly } from '../api/client'

export function problemText(e: unknown) {
  if (e instanceof ProblemError) {
    const errs = e.problem.errors?.map((x) => x.message || `${x.pointer} (${x.code})`).join(' · ')
    return errs || e.problem.detail || e.problem.title
  }
  return 'Speichern fehlgeschlagen.'
}

export interface Confirm { confirm_anomalies?: string[]; anomaly_reason?: string }

export function usePlausibleMutation<T>(fn: (extra: Confirm) => Promise<T>, onSuccess: (r: T) => void) {
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const m = useMutation({
    mutationFn: fn,
    onMutate: () => { setError(undefined) },
    onSuccess: (r) => { setAnomalies(null); onSuccess(r) },
    onError: (e) => {
      if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
      else setError(problemText(e))
    },
  })
  return {
    submit: () => { setAnomalies(null); m.mutate({}) },
    confirm: (codes: string[], reason: string) => m.mutate({ confirm_anomalies: codes, anomaly_reason: reason }),
    anomalies, error, busy: m.isPending,
    reset: () => { setAnomalies(null); setError(undefined) },
  }
}
