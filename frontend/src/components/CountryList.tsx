import { useId } from 'react'
import type { DailyClicks } from '../api/types.ts'
import { clicksLabel, formatCount } from '../format.ts'

// Up to this many countries get their own row; the rest add up in one.
const shownCountries = 8

const regionNames = new Intl.DisplayNames(['pt-BR'], { type: 'region' })

function countryName(code: string): string {
  try {
    return regionNames.of(code) ?? code
  } catch {
    return code
  }
}

// The flag emoji is the code written in regional indicator letters.
function flag(code: string): string {
  return String.fromCodePoint(...[...code].map((letter) => 0x1f1e6 + letter.charCodeAt(0) - 65))
}

type Row = { key: string; label: string; flag?: string; clicks: number; muted?: boolean }

function rowsOf(data: DailyClicks): Row[] {
  const rows: Row[] = data.countries
    .slice(0, shownCountries)
    .map((country) => ({ key: country.country, label: countryName(country.country), flag: flag(country.country), clicks: country.clicks }))
  const others = data.countries.slice(shownCountries)
  if (others.length > 0) {
    rows.push({
      key: 'others',
      label: `Outros ${formatCount(others.length)} países`,
      clicks: others.reduce((sum, country) => sum + country.clicks, 0),
    })
  }
  if (data.totals.no_country > 0) {
    rows.push({ key: 'unknown', label: 'País não identificado', clicks: data.totals.no_country, muted: true })
  }
  return rows
}

export function CountryList({ data }: { data: DailyClicks }) {
  const titleId = useId()
  const rows = rowsOf(data)
  const top = Math.max(0, ...rows.map((row) => row.clicks))

  return (
    <section aria-labelledby={titleId} className="mt-8 border-t pt-6">
      <h3 id={titleId} className="text-base font-bold">
        Países
      </h3>
      {rows.length === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">Nenhum acesso neste período.</p>
      ) : (
        <ul className="mt-4 grid gap-3" aria-labelledby={titleId}>
          {rows.map((row) => (
            <li key={row.key} className="grid gap-1.5">
              <div className="flex items-baseline justify-between gap-3 text-sm">
                <span className={row.muted ? 'text-muted-foreground' : 'font-medium text-navy'}>
                  {row.flag && (
                    <span className="mr-2" aria-hidden="true">
                      {row.flag}
                    </span>
                  )}
                  {row.label}
                </span>
                <span className="shrink-0 tabular-nums">
                  <span className="font-bold text-navy">{formatCount(row.clicks)}</span>{' '}
                  <span className="text-muted-foreground">{clicksLabel(row.clicks)}</span>
                </span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-surface" aria-hidden="true">
                <div
                  className={row.muted ? 'h-full rounded-full bg-navy/20' : 'h-full rounded-full bg-action'}
                  style={{ width: `${top === 0 ? 0 : (row.clicks / top) * 100}%` }}
                />
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
