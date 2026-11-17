import { useMemo, useState } from 'react'
import type { ClickPeriod, DailyClicks, DayClicks } from '../../api/types.ts'
import { ClickStats, ClickStatsCard } from '../ClickChart.tsx'

// Share of the clicks from each country in the example; the rest has no
// country, as clicks whose origin is not identified.
const countryShares: [string, number][] = [
  ['BR', 0.62],
  ['PT', 0.12],
  ['US', 0.08],
  ['AR', 0.05],
  ['MX', 0.03],
  ['ES', 0.02],
  ['FR', 0.015],
  ['DE', 0.01],
  ['IT', 0.008],
  ['JP', 0.005],
]

const longestPeriod = 90

// A small seeded generator, so the example shows the same numbers on every
// visit and between renders.
function seeded(seed: number): () => number {
  let state = seed
  return () => {
    state = (state + 0x6d2b79f5) | 0
    let value = Math.imul(state ^ (state >>> 15), 1 | state)
    value = (value + Math.imul(value ^ (value >>> 7), 61 | value)) ^ value
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296
  }
}

// Ninety days ending today: a slow rise, busier weekdays and one day the link
// went around, split between the link and its QR code.
function exampleDays(today: Date): DayClicks[] {
  const random = seeded(2026)
  const end = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
  return Array.from({ length: longestPeriod }, (_, i) => {
    const date = new Date(end - (longestPeriod - 1 - i) * 86_400_000)
    const weekday = date.getUTCDay()
    const trend = 14 + i * 0.35
    const week = weekday === 0 || weekday === 6 ? 0.7 : 1.1
    const spike = i === longestPeriod - 12 ? 2.6 : 1
    const total = Math.round(trend * week * spike * (0.75 + random() * 0.5))
    const qr = Math.round(total * (0.22 + random() * 0.12))
    return {
      date: date.toISOString().slice(0, 10),
      link: total - qr,
      qr,
      visitors: Math.round(total * (0.72 + random() * 0.12)),
      robots: Math.round(random() * 4),
    }
  })
}

function exampleStats(days: DayClicks[]): DailyClicks {
  const sum = (key: 'link' | 'qr' | 'visitors' | 'robots') => days.reduce((total, day) => total + day[key], 0)
  const clicks = sum('link') + sum('qr')
  const countries = countryShares.map(([country, share]) => ({ country, clicks: Math.round(clicks * share) }))
  const withCountry = countries.reduce((total, country) => total + country.clicks, 0)
  return {
    days,
    totals: { link: sum('link'), qr: sum('qr'), visitors: sum('visitors'), robots: sum('robots'), no_country: Math.max(0, clicks - withCountry) },
    countries,
  }
}

// The numbers of a link's page, with example data: the same chart, totals
// and countries the owner sees, period switch and day tooltips included.
export function ClickStatsShowcase() {
  const [period, setPeriod] = useState<ClickPeriod>(30)
  // Computed once, so the example ends today and stays put while the page is open.
  const [days] = useState(() => exampleDays(new Date()))
  const data = useMemo(() => exampleStats(days.slice(-period)), [days, period])

  return (
    <div className="mx-auto mt-10 w-full max-w-5xl">
      <ClickStatsCard
        period={period}
        onPeriodChange={setPeriod}
        headingLevel="h3"
        badge={<span className="rounded-full bg-brand/15 px-2.5 py-0.5 text-xs font-bold text-brand-700">Exemplo</span>}
      >
        <ClickStats data={data} />
      </ClickStatsCard>
    </div>
  )
}
