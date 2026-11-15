export class ApiError extends Error {
  readonly status: number
  // Stable name of the error from the API's problem document, such as
  // "password_too_short"; empty when the answer was not one.
  readonly code: string

  constructor(status: number, message: string, code = '') {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const networkErrorMessage = 'Não foi possível falar com o servidor. Verifique sua conexão e tente de novo.'
const serverErrorMessage = 'Algo deu errado do nosso lado. Tente de novo em instantes.'
const rateLimitMessage = 'Muitas tentativas seguidas. Aguarde um pouco e tente de novo.'
const fallbackMessage = 'Não foi possível concluir a ação.'

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

// The API answers errors as RFC 9457 problem documents: detail is a
// Portuguese message meant for people and code a stable name. Anything else,
// such as a proxy's error page, is read as plain text.
export function parseProblem(body: string): { detail: string; code: string } {
  try {
    const parsed: unknown = JSON.parse(body)
    if (parsed && typeof parsed === 'object' && 'detail' in parsed && typeof parsed.detail === 'string') {
      const code = 'code' in parsed && typeof parsed.code === 'string' ? parsed.code : ''
      return { detail: parsed.detail, code }
    }
  } catch {
    // Not JSON: fall through to plain text.
  }
  return { detail: body, code: '' }
}

// Server errors are replaced so internal wording never reaches the screen.
export function errorMessageFor(status: number, detail: string): string {
  if (status >= 500) {
    return serverErrorMessage
  }
  if (detail) {
    return capitalize(detail)
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
    const { detail, code } = parseProblem(text)
    throw new ApiError(response.status, errorMessageFor(response.status, detail), code)
  }

  return (text ? JSON.parse(text) : undefined) as T
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

export function messageOf(error: unknown): string {
  return error instanceof ApiError ? error.message : serverErrorMessage
}
