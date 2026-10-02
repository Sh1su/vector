import type { Schemas } from '../api/client'
import { fmtDate, fmtNumber } from './format'

export const levelInfo: Record<string, { label: string; tone: 'ok' | 'warn' | 'bad' | 'info' | 'neutral' }> = {
  overdue: { label: 'überfällig', tone: 'bad' }, due: { label: 'fällig', tone: 'warn' }, upcoming: { label: 'demnächst', tone: 'info' },
  ok: { label: 'ok', tone: 'ok' }, unknown: { label: 'unbekannt', tone: 'neutral' }, completed: { label: 'erledigt', tone: 'ok' },
}


export function dueText(s?: Schemas['DueStatus']) {
  if (!s) return ''
  const parts: string[] = []
  if (s.days_remaining != null && s.due_date) parts.push(s.days_remaining < 0 ? `seit ${-s.days_remaining} Tagen (${fmtDate(s.due_date, true)})` : s.days_remaining === 0 ? 'heute' : `in ${s.days_remaining} Tagen (${fmtDate(s.due_date, true)})`)
  if (s.distance_remaining) parts.push(s.distance_remaining.value < 0 ? `${fmtNumber(-s.distance_remaining.value)} ${s.distance_remaining.unit} drüber` : `noch ${fmtNumber(s.distance_remaining.value)} ${s.distance_remaining.unit}`)
  else if (s.due_total) parts.push(`bei ${fmtNumber(s.due_total.canonical / 1000)} km`)
  if (s.level === 'unknown') parts.push('Kilometerstand erfassen')
  return parts.join(' · ')
}

