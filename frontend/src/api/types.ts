export type User = {
  id: string
  email: string
}

export type Account = User & {
  full_name: string | null
  has_password: boolean
  has_google: boolean
  created_at: string
}

export type Visibility = 'public' | 'private'

export type ShortLink = {
  short_code: string
  original_url: string
  visibility: Visibility
  click_count: number
  created_at: string
  updated_at: string
}

export type ShortLinkPage = {
  urls: ShortLink[]
  next_cursor: string | null
}

export type CreatedLink = {
  short_code: string
  short_url: string
}

export type NewLink =
  | { url: string; visibility: 'public' }
  | { url: string; visibility: 'private'; password: string }

// Clicks of one day: by people opening the short link and by people
// scanning its QR code.
export type DayClicks = {
  date: string
  link: number
  qr: number
}

export type DailyClicks = {
  days: DayClicks[]
  totals: { link: number; qr: number }
}

export type ClickPeriod = 7 | 30 | 90
