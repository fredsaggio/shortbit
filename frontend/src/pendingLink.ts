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

export function takePendingLink(): string {
  try {
    const url = sessionStorage.getItem(key) ?? ''
    sessionStorage.removeItem(key)
    return url
  } catch {
    return ''
  }
}
