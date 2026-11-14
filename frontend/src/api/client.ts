export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

const networkErrorMessage = 'Não foi possível falar com o servidor. Verifique sua conexão e tente de novo.'
const serverErrorMessage = 'Algo deu errado do nosso lado. Tente de novo em instantes.'
const rateLimitMessage = 'Muitas tentativas seguidas. Aguarde um pouco e tente de novo.'
const fallbackMessage = 'Não foi possível concluir a ação.'

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

// The API answers errors with Portuguese plain text meant for people. Server
// errors are replaced so internal wording never reaches the screen.
export function errorMessageFor(status: number, body: string): string {
  if (status >= 500) {
    return serverErrorMessage
  }
  if (body) {
    return capitalize(body)
  }
  return status === 429 ? rateLimitMessage : fallbackMessage
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  signal?: AbortSignal
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, signal } = options
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  let response: Response
  try {
    response = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
      signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw error
    }
    throw new ApiError(0, networkErrorMessage)
  }

  const text = (await response.text()).trim()

  if (!response.ok) {
    throw new ApiError(response.status, errorMessageFor(response.status, text))
  }

  return (text ? JSON.parse(text) : undefined) as T
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

export function messageOf(error: unknown): string {
  return error instanceof ApiError ? error.message : serverErrorMessage
}
