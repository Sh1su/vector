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

export const api = {
  get: <T,>(p: string) => request<T>('GET', p).then((r) => r.data),
  getWithETag: <T,>(p: string) => request<T>('GET', p),
  post: <T,>(p: string, b?: unknown, h?: Record<string, string>) => request<T>('POST', p, b ?? {}, h).then((r) => r.data),
  patch: <T,>(p: string, b: unknown, etag: string) => request<T>('PATCH', p, b, { 'If-Match': etag }).then((r) => r.data),
  del: (p: string, etag: string) => request<void>('DELETE', p, undefined, { 'If-Match': etag }),
}
