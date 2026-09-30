// Schlanker Fetch-Wrapper (ADR-022): Typen aus der OpenAPI, CSRF-Header,
// ETag/If-Match und Problem Details (ADR-013).
import type { components } from './schema'

export type Schemas = components['schemas']
export type Problem = Schemas['Problem']
export type Anomaly = Schemas['Anomaly']

export class ProblemError extends Error {
  status: number
  problem: Problem
  constructor(problem: Problem) {
    super(problem.detail || problem.title)
    this.status = problem.status
    this.problem = problem
  }
  get isPlausibility() {
    return this.status === 422 && (this.problem.anomalies?.length ?? 0) > 0
  }
}

function csrfToken(): string {
  const m = document.cookie.match(/(?:^|;\s*)vectra_csrf=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : ''
}

export interface Result<T> {
  data: T
  etag: string | null
}

export async function request<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<Result<T>> {
  const h: Record<string, string> = { Accept: 'application/json', ...headers }
  if (body !== undefined) h['Content-Type'] = method === 'PATCH' ? 'application/merge-patch+json' : 'application/json'
  if (method !== 'GET') h['X-CSRF-Token'] = csrfToken()
  const res = await fetch('/api/v1' + path, { method, headers: h, body: body === undefined ? undefined : JSON.stringify(body), credentials: 'same-origin' })
  const text = await res.text()
  const json = text ? JSON.parse(text) : undefined
  if (!res.ok) {
    throw new ProblemError(json ?? { status: res.status, title: res.statusText, type: 'about:blank' })
  }
  return { data: json as T, etag: res.headers.get('ETag') }
}

/** Datei-Upload als multipart/form-data; Felder stehen vor der Datei (Server streamt die Datei). */
export async function upload<T>(path: string, file: Blob, name: string, fields: Record<string, string> = {}): Promise<{ data: T; status: number }> {
  const fd = new FormData()
  for (const [k, v] of Object.entries(fields)) fd.append(k, v)
  fd.append('file', file, name)
  const res = await fetch('/api/v1' + path, { method: 'POST', body: fd, credentials: 'same-origin', headers: { 'X-CSRF-Token': csrfToken(), Accept: 'application/json' } })
  const text = await res.text()
  const json = text ? JSON.parse(text) : undefined
  if (!res.ok) throw new ProblemError(json ?? { status: res.status, title: res.statusText, type: 'about:blank' })
  return { data: json as T, status: res.status }
}

export const api = {
  get: <T,>(p: string) => request<T>('GET', p).then((r) => r.data),
  getWithETag: <T,>(p: string) => request<T>('GET', p),
  post: <T,>(p: string, b?: unknown, h?: Record<string, string>) => request<T>('POST', p, b ?? {}, h).then((r) => r.data),
  patch: <T,>(p: string, b: unknown, etag: string) => request<T>('PATCH', p, b, { 'If-Match': etag }).then((r) => r.data),
  del: (p: string, etag: string) => request<void>('DELETE', p, undefined, { 'If-Match': etag }),
}
