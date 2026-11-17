import { useEffect, useId, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { isAbortError, messageOf } from '../api/client.ts'
import { getDailyClicks } from '../api/links.ts'
import type { ClickPeriod, DailyClicks, DayClicks } from '../api/types.ts'
import { clicksLabel, formatCount, formatLongDay, formatShortDay } from '../format.ts'
import { Alert } from './Alert.tsx'
import { CountryList } from './CountryList.tsx'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const periods: ClickPeriod[] = [7, 30, 90]

const periodItemClass =
  'h-9 flex-1 rounded-md px-3 text-sm font-semibold text-muted-foreground transition-colors hover:bg-surface hover:text-navy data-[state=on]:bg-navy data-[state=on]:text-white data-[state=on]:shadow-sm sm:flex-none'

// Link and QR code: the site's action blue and brand orange, checked for
// color-blind separation and contrast against the card.
const sources = [
  { key: 'link', label: 'Pelo link', inline: 'pelo link', fill: 'bg-action' },
  { key: 'qr', label: 'Pelo QR code', inline: 'pelo QR code', fill: 'bg-brand' },
] as const

// Each answer remembers the request it came from, so a newer request in
// flight shows as "refreshing" without extra state.
type ChartState =
  | { status: 'loading' }
  | { status: 'ready'; data: DailyClicks; request: string }
  | { status: 'error'; error: string; request: string }

// A clean axis: a step of 1, 2 or 5 times a power of ten giving about four
// gridlines, never below one click.
function axisTicks(max: number): number[] {
  if (max <= 0) {
    return [0]
  }
  const rough = Math.max(1, max / 4)
  const power = 10 ** Math.floor(Math.log10(rough))
  const step = [1, 2, 5, 10].map((m) => m * power).find((candidate) => candidate >= rough) ?? 10 * power
  const top = Math.ceil(max / step) * step
  return Array.from({ length: top / step + 1 }, (_, i) => i * step)
}

function dayTotal(day: DayClicks): number {
  return day.link + day.qr
}

function visitorsLabel(value: number): string {
  return value === 1 ? 'visitante único' : 'visitantes únicos'
}

function robotsLabel(value: number): string {
  return value === 1 ? 'prévia de robô' : 'prévias de robôs'
}

function daySummary(day: DayClicks): string {
  const total = dayTotal(day)
  const robots = day.robots > 0 ? `, ${formatCount(day.robots)} ${robotsLabel(day.robots)}` : ''
  return `${formatLongDay(day.date)}: ${formatCount(total)} ${clicksLabel(total)}, ${formatCount(day.link)} pelo link e ${formatCount(day.qr)} pelo QR code, ${formatCount(day.visitors)} ${visitorsLabel(day.visitors)}${robots}`
}

type ClickChartProps = {
  code: string
  // Changes when the page asks for fresh numbers.
  reloadKey: number
}

export function ClickChart({ code, reloadKey }: ClickChartProps) {
  const titleId = useId()
  const [period, setPeriod] = useState<ClickPeriod>(30)
  const [state, setState] = useState<ChartState>({ status: 'loading' })

  const request = `${code}:${period}:${reloadKey}`
  // A reload keeps the previous chart on screen, dimmed, instead of a skeleton.
  const refreshing = state.status !== 'loading' && state.request !== request

  useEffect(() => {
    const controller = new AbortController()
    const current = `${code}:${period}:${reloadKey}`
    getDailyClicks(code, period, controller.signal).then(
      (data) => setState({ status: 'ready', data, request: current }),
      (error: unknown) => {
        if (!isAbortError(error)) {
          setState({ status: 'error', error: messageOf(error), request: current })
        }
      },
    )
    return () => controller.abort()
  }, [code, period, reloadKey])

  return (
    <section aria-labelledby={titleId} className="rounded-3xl border bg-card p-6 shadow-card sm:p-8">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 id={titleId} className="text-lg font-bold">
          Cliques por dia
        </h2>
        <ToggleGroup
          type="single"
          value={String(period)}
          onValueChange={(next) => {
            if (next) {
              setPeriod(Number(next) as ClickPeriod)
            }
          }}
          aria-label="Período"
          className="flex w-full gap-1 rounded-lg border bg-surface p-1 sm:w-fit"
        >
          {periods.map((days) => (
            <ToggleGroupItem key={days} value={String(days)} className={periodItemClass}>
              {days} dias
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>

      {state.status === 'loading' && <Skeleton className="mt-6 h-64 rounded-xl bg-surface-200" role="status" aria-label="Carregando os cliques…" />}
      {state.status === 'error' && (
        <Alert tone="error" className="mt-6">
          {state.error}
        </Alert>
      )}
      {state.status === 'ready' && (
        <div className={cn('transition-opacity duration-200', refreshing && 'opacity-50')} aria-busy={refreshing}>
          <ChartLegend data={state.data} />
          <ChartPlot days={state.data.days} />
          <ChartTable days={state.data.days} />
          <CountryList data={state.data} />
        </div>
      )}
    </section>
  )
}

function ChartLegend({ data }: { data: DailyClicks }) {
  return (
    <>
      <ul className="mt-5 flex flex-wrap gap-x-6 gap-y-2" aria-label="Total no período">
        {sources.map((source) => (
          <li key={source.key} className="flex items-center gap-2">
            <span className={cn('size-3 rounded-[3px]', source.fill)} aria-hidden="true" />
            <span className="text-sm text-muted-foreground">{source.label}</span>
            <span className="font-bold text-navy">{formatCount(data.totals[source.key])}</span>
          </li>
        ))}
        <li className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Visitantes únicos</span>
          <span className="font-bold text-navy">{formatCount(data.totals.visitors)}</span>
        </li>
        <li className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Prévias de robôs</span>
          <span className="font-bold text-navy">{formatCount(data.totals.robots)}</span>
        </li>
      </ul>
      <p className="mt-2 text-xs text-muted-foreground">
        Os visitantes únicos somam os de cada dia: quem volta em outro dia conta de novo. Prévias de robôs, como a que o
        WhatsApp monta ao receber o link, não contam como clique.
      </p>
    </>
  )
}

function ChartPlot({ days }: { days: DayClicks[] }) {
  const [active, setActive] = useState<number | null>(null)
  const ticks = axisTicks(Math.max(0, ...days.map(dayTotal)))
  const top = ticks[ticks.length - 1]
  const empty = top === 0
  const activeDay = active === null ? null : days[active]
  const middle = Math.floor((days.length - 1) / 2)

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const last = days.length - 1
    const moves: Record<string, (current: number) => number> = {
      ArrowLeft: (current) => Math.max(0, current - 1),
      ArrowRight: (current) => Math.min(last, current + 1),
      Home: () => 0,
      End: () => last,
    }
    const move = moves[event.key]
    if (move) {
      event.preventDefault()
      setActive((current) => move(current ?? last))
    }
  }

  return (
    <div className="mt-5 flex gap-2">
      {/* Y axis: clean steps, hairline gridlines behind the columns. */}
      <div className="relative mt-2 h-52 w-8 shrink-0 text-right text-xs text-muted-foreground tabular-nums" aria-hidden="true">
        {!empty &&
          ticks.map((tick) => (
            <span key={tick} className="absolute right-0 translate-y-1/2 leading-none" style={{ bottom: `${(tick / top) * 100}%` }}>
              {formatCount(tick)}
            </span>
          ))}
      </div>

      <div className="min-w-0 flex-1 pt-2">
        <div>
          <div
            role="group"
            tabIndex={0}
            aria-label="Gráfico de cliques por dia. Use as setas para ver cada dia."
            onKeyDown={onKeyDown}
            onFocus={() => setActive((current) => current ?? days.length - 1)}
            onBlur={() => setActive(null)}
            onPointerLeave={() => setActive(null)}
            className="relative h-52 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-action focus-visible:ring-offset-4"
          >
            {ticks.map((tick) => (
              <div
                key={tick}
                aria-hidden="true"
                className={cn('absolute inset-x-0 h-px', tick === 0 ? 'bg-navy/25' : 'bg-border/70')}
                style={{ bottom: `${top === 0 ? 0 : (tick / top) * 100}%` }}
              />
            ))}

            {empty && (
              <p className="absolute inset-0 grid place-items-center text-sm text-muted-foreground">Nenhum clique neste período.</p>
            )}

            {/* Each day is a full-height hit area; the 2px gap between days and
                between the two parts of a column is the card showing through. */}
            {/* 90 days on a phone leave about 3px per day: the gap drops to 1px there. */}
            <div className={cn('absolute inset-0 flex', days.length > 30 ? 'gap-px sm:gap-[2px]' : 'gap-[2px]')} aria-hidden="true">
              {days.map((day, index) => {
                const total = dayTotal(day)
                return (
                  <div
                    key={day.date}
                    onPointerEnter={() => setActive(index)}
                    className={cn('flex h-full flex-1 flex-col-reverse items-center rounded-sm transition-colors', active === index && 'bg-navy/5')}
                  >
                    {total > 0 && (
                      <div className="flex w-full max-w-6 flex-col-reverse gap-[2px]" style={{ height: `${(total / top) * 100}%` }}>
                        {day.link > 0 && (
                          <div className={cn('w-full bg-action', day.qr === 0 && 'rounded-t-[4px]')} style={{ flexGrow: day.link }} />
                        )}
                        {day.qr > 0 && <div className="w-full rounded-t-[4px] bg-brand" style={{ flexGrow: day.qr }} />}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>

            {activeDay && active !== null && <ChartTooltip day={activeDay} position={(active + 0.5) / days.length} />}
          </div>

          {/* A few dates are enough: first, middle and today. */}
          <div className="relative mt-2 h-4 text-xs text-muted-foreground" aria-hidden="true">
            {[0, middle, days.length - 1]
              .filter((index, i, all) => all.indexOf(index) === i)
              .map((index) => (
                <span
                  key={index}
                  className={cn('absolute whitespace-nowrap', index === 0 ? 'left-0' : index === days.length - 1 ? 'right-0' : '-translate-x-1/2')}
                  style={index === middle && index !== 0 && index !== days.length - 1 ? { left: `${((index + 0.5) / days.length) * 100}%` } : undefined}
                >
                  {formatShortDay(days[index].date)}
                </span>
              ))}
          </div>
        </div>
      </div>

      {/* Screen readers hear the day the arrows land on. */}
      <p className="sr-only" aria-live="polite">
        {activeDay ? daySummary(activeDay) : ''}
      </p>
    </div>
  )
}

function ChartTooltip({ day, position }: { day: DayClicks; position: number }) {
  // Kept inside the chart: anchored left near the start, right near the end.
  const anchor = position < 0.25 ? 'left' : position > 0.75 ? 'right' : 'center'
  return (
    <div
      aria-hidden="true"
      className={cn(
        'pointer-events-none absolute top-0 z-10 w-max rounded-lg border bg-card px-3 py-2 text-sm shadow-raised',
        anchor === 'center' && '-translate-x-1/2',
        anchor === 'right' && '-translate-x-full',
      )}
      style={{ left: `${position * 100}%` }}
    >
      <p className="text-xs text-muted-foreground">{formatLongDay(day.date)}</p>
      {sources.map((source) => (
        <p key={source.key} className="mt-1 flex items-center gap-2">
          <span className={cn('h-0.5 w-3 rounded-full', source.fill)} />
          <span className="font-bold text-navy tabular-nums">{formatCount(day[source.key])}</span>
          <span className="text-muted-foreground">{source.inline}</span>
        </p>
      ))}
      <p className="mt-1 flex items-center gap-2 border-t pt-1">
        <span className="font-bold text-navy tabular-nums">{formatCount(day.visitors)}</span>
        <span className="text-muted-foreground">{visitorsLabel(day.visitors)}</span>
      </p>
      {day.robots > 0 && (
        <p className="mt-1 flex items-center gap-2">
          <span className="font-bold text-navy tabular-nums">{formatCount(day.robots)}</span>
          <span className="text-muted-foreground">{robotsLabel(day.robots)}</span>
        </p>
      )}
    </div>
  )
}

function ChartTable({ days }: { days: DayClicks[] }) {
  const withClicks = [...days].reverse().filter((day) => dayTotal(day) > 0 || day.robots > 0)
  return (
    <details className="group mt-5 text-sm">
      <summary className="inline-flex cursor-pointer items-center gap-1 rounded-md font-semibold text-action hover:underline">
        Ver os números em tabela
      </summary>
      {withClicks.length === 0 ? (
        <p className="mt-3 text-muted-foreground">Nenhum clique neste período.</p>
      ) : (
        <div className="mt-3 max-h-72 overflow-auto rounded-lg border">
          <table className="w-full text-left">
            <caption className="sr-only">Cliques por dia, do mais recente ao mais antigo; dias sem clique nem prévia ficam de fora</caption>
            <thead className="sticky top-0 bg-surface text-muted-foreground">
              <tr>
                <th scope="col" className="px-3 py-2 font-medium">Dia</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Pelo link</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Pelo QR code</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Total</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Visitantes únicos</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Prévias de robôs</th>
              </tr>
            </thead>
            <tbody className="tabular-nums">
              {withClicks.map((day) => (
                <tr key={day.date} className="border-t">
                  <th scope="row" className="px-3 py-2 font-medium whitespace-nowrap">{formatLongDay(day.date)}</th>
                  <td className="px-3 py-2 text-right">{formatCount(day.link)}</td>
                  <td className="px-3 py-2 text-right">{formatCount(day.qr)}</td>
                  <td className="px-3 py-2 text-right font-semibold">{formatCount(dayTotal(day))}</td>
                  <td className="px-3 py-2 text-right">{formatCount(day.visitors)}</td>
                  <td className="px-3 py-2 text-right">{formatCount(day.robots)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </details>
  )
}
