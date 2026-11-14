import { describe, expect, it, vi } from 'vitest'
import { ApiError, apiRequest, errorMessageFor } from './client.ts'

describe('errorMessageFor', () => {
  it('capitalizes the API message for client errors', () => {
    expect(errorMessageFor(401, 'email ou senha incorretos')).toBe('Email ou senha incorretos')
  })

  it('hides server error details', () => {
    expect(errorMessageFor(500, 'pq: connection refused')).toBe('Algo deu errado do nosso lado. Tente de novo em instantes.')
  })

  it('explains an empty rate limit response', () => {
    expect(errorMessageFor(429, '')).toBe('Muitas tentativas seguidas. Aguarde um pouco e tente de novo.')
  })
})

describe('apiRequest', () => {
  it('sends JSON and parses the JSON answer', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ short_code: 'Ab3dX9' }), { status: 201 }))
    vi.stubGlobal('fetch', fetchMock)

    const result = await apiRequest<{ short_code: string }>('/urls', { method: 'POST', body: { url: 'https://example.com' } })

    expect(result).toEqual({ short_code: 'Ab3dX9' })
    expect(fetchMock).toHaveBeenCalledWith('/urls', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ url: 'https://example.com' }),
      credentials: 'same-origin',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    }))
  })

  it('returns undefined for an empty success', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 204 })))

    await expect(apiRequest('/sessions', { method: 'POST', body: {} })).resolves.toBeUndefined()
  })

  it('turns an error status into an ApiError with the API message', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('email já está em uso\n', { status: 409 })))

    const error = await apiRequest('/registrations/password').catch((caught: unknown) => caught)

    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 409, message: 'Email já está em uso' })
  })

  it('reports a network failure without leaking the browser error', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('Failed to fetch'))))

    await expect(apiRequest('/me')).rejects.toMatchObject({
      status: 0,
      message: 'Não foi possível falar com o servidor. Verifique sua conexão e tente de novo.',
    })
  })
})
