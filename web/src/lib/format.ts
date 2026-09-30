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
