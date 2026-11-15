import { vi } from 'vitest'

export type FakeRequest = {
  method: string
  path: string
  body: unknown
}

type FakeResponse = { status: number; body?: unknown; code?: string }
type Handler = FakeResponse | ((request: FakeRequest) => FakeResponse)

// Routes are keyed as "METHOD /path?query". A request without a matching
// route fails the test with a 599, so unexpected calls are visible.
export function fakeApi(routes: Record<string, Handler>) {
  const requests: FakeRequest[] = []

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const path = String(input)
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
    const request = { method, path, body }
    requests.push(request)

    const handler = routes[`${method} ${path}`]
    if (!handler) {
      return new Response(`rota não simulada: ${method} ${path}`, { status: 599 })
    }

    const response = typeof handler === 'function' ? handler(request) : handler
    // A string body on an error is the message; send it the way the API
    // does, inside a problem document.
    if (response.status >= 400 && typeof response.body === 'string') {
      const problem = { type: 'about:blank', title: '', status: response.status, detail: response.body, code: response.code ?? 'test_error' }
      return new Response(JSON.stringify(problem), { status: response.status, headers: { 'Content-Type': 'application/problem+json' } })
    }
    const payload = response.body === undefined ? null : JSON.stringify(response.body)
    return new Response(payload, { status: response.status })
  })

  vi.stubGlobal('fetch', fetchMock)

  return {
    requests,
    calls: (method: string, path: string) => requests.filter((request) => request.method === method && request.path === path),
  }
}

export const signedOut = { 'GET /me': { status: 401, body: 'autenticação necessária' } }
export const user = {
  id: '0194d3b2-0000-7000-8000-000000000001',
  email: 'ana@example.com',
  full_name: 'Ana Souza',
  has_password: true,
  has_google: false,
  created_at: '2026-11-14T12:00:00Z',
}

// Daily clicks as the API answers them: one entry per day ending on
// 2026-11-14, zero unless given.
export function dailyClicks(days: number, counts: Record<string, { link?: number; qr?: number }> = {}) {
  const end = Date.UTC(2026, 10, 14)
  const entries = Array.from({ length: days }, (_, i) => {
    const date = new Date(end - (days - 1 - i) * 86_400_000).toISOString().slice(0, 10)
    return { date, link: counts[date]?.link ?? 0, qr: counts[date]?.qr ?? 0 }
  })
  return {
    days: entries,
    totals: {
      link: entries.reduce((sum, day) => sum + day.link, 0),
      qr: entries.reduce((sum, day) => sum + day.qr, 0),
    },
  }
}
