// A URL typed on the home page before signing in waits here, per tab, and
// fills the dashboard form after login. Storage can be unavailable (private
// mode, blocked site data), so every access is guarded.
const key = 'shortbit:pending-url'

export function savePendingLink(url: string): void {
  try {
    sessionStorage.setItem(key, url)
  } catch {
    // Without storage the visitor just types the URL again.
  }
}

// Reading and clearing are separate because React may call a state
// initializer twice in development.
export function readPendingLink(): string {
  try {
    return sessionStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

export function clearPendingLink(): void {
  try {
    sessionStorage.removeItem(key)
  } catch {
    // Nothing to clear without storage.
  }
}
