// Formatierung nur für die Anzeige (ADR-028); die API bleibt sprachunabhängig.
const nf = (d: number) => new Intl.NumberFormat('de-DE', { maximumFractionDigits: d, minimumFractionDigits: 0 })

export const fmtNumber = (v: number, digits = 0) => nf(digits).format(v)
export const fmtDate = (iso: string, dateOnly = false) =>
  new Intl.DateTimeFormat('de-DE', dateOnly ? { dateStyle: 'medium' } : { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
export const fmtDistance = (canonicalMeters: number, unit = 'km') =>
  `${fmtNumber(unit === 'mi' ? canonicalMeters / 1609.344 : canonicalMeters / 1000)} ${unit}`

/** Liefert „YYYY-MM-DDTHH:mm“ in lokaler Zeit für <input type="datetime-local">. */
export function localInputValue(d = new Date()) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export const browserTimeZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Berlin'

export const fmtMoney = (minor: number, currency = 'EUR') =>
  new Intl.NumberFormat('de-DE', { style: 'currency', currency }).format(minor / 100)

/** Wert mit Einheit aus einem DisplayValue der API. */
export const fmtDisplay = (d?: { value: number; unit: string } | null, digits = 2) => (d ? `${fmtNumber(d.value, digits)} ${d.unit}` : '–')

/** Dezimaleingabe mit Komma oder Punkt. */
export const num = (s: string) => Number(s.includes(',') ? s.replace(/\./g, '').replace(',', '.') : s)

/** Zeitpunkt aus datetime-local bzw. date (12:00 lokal) als ISO-String. */
export function toIso(at: string, dateOnly: boolean) {
  return (dateOnly ? new Date(at.slice(0, 10) + 'T12:00') : new Date(at)).toISOString()
}

export const today = () => localInputValue().slice(0, 10)

export const carrierLabel: Record<string, string> = { petrol: 'Benzin', diesel: 'Diesel', lpg: 'Autogas', electricity: 'Strom' }
