import { vi } from 'vitest'

export type FakeRequest = {
  method: string
  path: string
  body: unknown
}

type FakeResponse = { status: number; body?: unknown }
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
    const payload =
      response.body === undefined ? null : typeof response.body === 'string' ? response.body : JSON.stringify(response.body)
    return new Response(payload, { status: response.status })
  })

  vi.stubGlobal('fetch', fetchMock)

  return {
    requests,
    calls: (method: string, path: string) => requests.filter((request) => request.method === method && request.path === path),
  }
}

export const signedOut = { 'GET /me': { status: 401, body: 'autenticação necessária' } }
export const user = { id: '0194d3b2-0000-7000-8000-000000000001', email: 'ana@example.com' }
