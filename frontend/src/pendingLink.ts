import type { Visibility } from './api/types.ts'

// A link asked for on the home page before signing in waits here and is
// created by the dashboard right after login.
//
// The URL and the visibility go to sessionStorage (per tab), so they survive
// the round trip to Google. The link password never does: web storage is
// readable by any script on the page (OWASP HTML5 Security Cheat Sheet), so it
// stays in memory. It survives navigation inside the app, as with email
// sign-in, but not a full page load; then the dashboard asks for it again.
const key = 'shortbit:pending-link'

export type PendingLink = {
  url: string
  visibility: Visibility
  /** Only for private links, and only while the page has not reloaded. */
  password?: string
  /** The visitor asked for the QR code: the dashboard opens it once the link exists. */
  qr?: boolean
}

let pendingPassword: string | undefined

export function savePendingLink(link: PendingLink): void {
  pendingPassword = link.visibility === 'private' ? link.password : undefined
  try {
    sessionStorage.setItem(key, JSON.stringify({ url: link.url, visibility: link.visibility, qr: link.qr === true }))
  } catch {
    // Without storage the visitor just types the URL again.
  }
}

function isStoredLink(value: unknown): value is { url: string; visibility: Visibility; qr?: boolean } {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const { url, visibility, qr } = value as Record<string, unknown>
  return (
    typeof url === 'string' &&
    url !== '' &&
    (visibility === 'public' || visibility === 'private') &&
    (qr === undefined || typeof qr === 'boolean')
  )
}

// Reading and clearing are separate because React may call a state
// initializer twice in development.
export function readPendingLink(): PendingLink | null {
  let stored: unknown
  try {
    const raw = sessionStorage.getItem(key)
    if (!raw) {
      return null
    }
    stored = JSON.parse(raw)
  } catch {
    return null
  }

  if (!isStoredLink(stored)) {
    return null
  }

  const qr = stored.qr === true
  return stored.visibility === 'private'
    ? { url: stored.url, visibility: 'private', password: pendingPassword, qr }
    : { url: stored.url, visibility: 'public', qr }
}

export function clearPendingLink(): void {
  pendingPassword = undefined
  try {
    sessionStorage.removeItem(key)
  } catch {
    // Nothing to clear without storage.
  }
}
