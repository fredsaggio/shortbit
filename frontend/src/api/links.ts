import { apiRequest } from './client.ts'
import type { CreatedLink, NewLink, ShortLink, ShortLinkPage } from './types.ts'

const shortLinkBaseURL = (import.meta.env.VITE_SHORT_LINK_BASE_URL || 'http://localhost:8080').replace(/\/+$/, '')

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
