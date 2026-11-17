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

// One day of a link: clicks by people opening the short link and by people
// scanning its QR code, the different people among them, and visits of link
// preview robots, which are not clicks.
export type DayClicks = {
  date: string
  link: number
  qr: number
  visitors: number
  robots: number
}

// Clicks of people from one country (ISO 3166-1 alpha-2 code).
export type CountryClicks = {
  country: string
  clicks: number
}

// Totals of the period. visitors adds up each day, so someone back on another
// day counts again; no_country is the clicks with an unknown country.
export type DailyClicks = {
  days: DayClicks[]
  totals: { link: number; qr: number; visitors: number; robots: number; no_country: number }
  countries: CountryClicks[]
}

export type ClickPeriod = 7 | 30 | 90
