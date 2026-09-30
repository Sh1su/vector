// Geldbeträge: API in kleinster Einheit (ADR-029), Anzeige in der Sprache der Oberfläche.
import { ProblemError } from '../api/client'

const zeroDigits = new Set(['JPY', 'KRW', 'ISK', 'CLP', 'VND', 'PYG', 'UGX', 'XAF', 'XOF', 'XPF', 'RWF', 'KMF', 'GNF', 'DJF', 'VUV'])
const threeDigits = new Set(['BHD', 'KWD', 'OMR', 'JOD', 'TND', 'LYD', 'IQD'])

export const minorDigits = (cur: string) => (zeroDigits.has(cur) ? 0 : threeDigits.has(cur) ? 3 : 2)

export function fmtMoney(minor: number, currency: string) {
  const d = minorDigits(currency)
  return new Intl.NumberFormat('de-DE', { style: 'currency', currency, minimumFractionDigits: d, maximumFractionDigits: d }).format(minor / 10 ** d)
}

/** Liest „1.234,56“ oder „1234.56“ als Betrag in kleinster Einheit; null bei ungültiger Eingabe. */
export function parseMoney(input: string, currency: string): number | null {
  let s = input.trim().replace(/\s|€/g, '')
  if (!s) return null
  if (s.includes(',')) s = s.replace(/\./g, '').replace(',', '.')
  const v = Number(s)
  if (!Number.isFinite(v)) return null
  return Math.round(v * 10 ** minorDigits(currency))
}

export const toMoneyInput = (minor: number, currency: string) =>
  (minor / 10 ** minorDigits(currency)).toFixed(minorDigits(currency)).replace('.', ',')

export function parseNumber(input: string): number | null {
  const s = input.trim().replace(/\./g, '').replace(',', '.')
  if (!s) return null
  const v = Number(s)
  return Number.isFinite(v) ? v : null
}

/** Verständliche Fehlermeldung aus Problem Details (ADR-013). */
export function errorText(e: unknown) {
  if (e instanceof ProblemError) {
    const errs = e.problem.errors?.map((x) => x.message || `${x.pointer} (${x.code})`).filter(Boolean)
    if (errs?.length) return errs.join(' · ')
    if (e.problem.anomalies?.length) return e.problem.anomalies.map((a) => a.message).join(' · ')
    return e.problem.detail || e.problem.title
  }
  return 'Speichern fehlgeschlagen.'
}

export const today = () => {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Datum „JJJJ-MM-TT“ als Zeitpunkt 12:00 lokal (date_only, ADR-008). */
export const noonISO = (date: string) => new Date(date + 'T12:00').toISOString()

export const fmtDay = (date: string) => new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium' }).format(new Date(date.length === 10 ? date + 'T12:00' : date))

export const etagOf = (v: { version?: number }) => `"${v.version ?? 1}"`

/** Kennzahl je Einheit (z. B. 0,53 € je km) mit mindestens zwei Nachkommastellen. */
export const fmtRate = (value: number, currency: string) =>
  new Intl.NumberFormat('de-DE', { style: 'currency', currency, minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value)
