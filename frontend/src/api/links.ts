import { apiRequest } from './client.ts'
import type { CreatedLink, NewLink, ShortLink, ShortLinkPage, Visibility } from './types.ts'

// In production the API serves this app, so short links share the page's
// origin unless the build says otherwise.
const shortLinkBaseURL = (
  import.meta.env.VITE_SHORT_LINK_BASE_URL || (import.meta.env.PROD ? window.location.origin : 'http://localhost:8080')
).replace(/\/+$/, '')

// The list and detail endpoints return only the code; the API builds short
// URLs from PUBLIC_BASE_URL, which VITE_SHORT_LINK_BASE_URL must match.
export function shortLinkURL(code: string): string {
  return `${shortLinkBaseURL}/${code}`
}

export function createLink(link: NewLink): Promise<CreatedLink> {
  return apiRequest<CreatedLink>('/urls', { method: 'POST', body: link })
}

export function listLinks(cursor: string | null, signal?: AbortSignal): Promise<ShortLinkPage> {
  const path = cursor ? `/urls?${new URLSearchParams({ cursor })}` : '/urls'
  return apiRequest<ShortLinkPage>(path, { signal })
}

export function getLink(code: string, signal?: AbortSignal): Promise<ShortLink> {
  return apiRequest<ShortLink>(`/urls/${encodeURIComponent(code)}`, { signal })
}

// Only the fields being changed. On a private link, an empty or absent
// password keeps the current one; a new one signs out whoever unlocked it.
export type LinkChanges = {
  url?: string
  visibility?: Visibility
  password?: string
}

export function updateLink(code: string, changes: LinkChanges): Promise<ShortLink> {
  return apiRequest<ShortLink>(`/urls/${encodeURIComponent(code)}`, { method: 'PATCH', body: changes })
}

export function deleteLink(code: string): Promise<void> {
  return apiRequest<void>(`/urls/${encodeURIComponent(code)}`, { method: 'DELETE' })
}
