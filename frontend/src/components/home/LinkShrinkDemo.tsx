import { ArrowRight } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import { formatCount } from '../../format.ts'
import { useCountUp, useInView, usePrefersReducedMotion } from '../../motion.ts'

// Illustrative examples, not real links or real numbers.
const examples = [
  {
    long: 'https://www.loja-exemplo.com.br/produtos/tenis-de-corrida-masculino?cor=azul&tamanho=42&utm_source=instagram',
    short: 'shortbit.com.br/Tn42Az',
    clicks: 1284,
  },
  {
    long: 'https://docs.exemplo.com/apresentacoes/d/1aBcD3eFgH4iJkL5mNoP6qRsT/edit?usp=sharing#slide=3',
    short: 'shortbit.com.br/Ap7Rs2',
    clicks: 356,
  },
  {
    long: 'https://www.exemplo.com.br/blog/2026/11/como-organizar-os-links-da-sua-campanha-de-black-friday',
    short: 'shortbit.com.br/Bf26Xq',
    clicks: 9821,
  },
]

type Phase = 'typing' | 'shrinking' | 'revealed' | 'counting' | 'holding' | 'leaving'

// How long each phase lasts before the next one, in milliseconds.
const phaseDuration: Record<Exclude<Phase, 'typing'>, number> = {
  shrinking: 700,
  revealed: 350,
  counting: 1300,
  holding: 1800,
  leaving: 400,
}
const nextPhase: Record<Exclude<Phase, 'typing'>, Phase> = {
  shrinking: 'revealed',
  revealed: 'counting',
  counting: 'holding',
  holding: 'leaving',
  leaving: 'typing',
}
const charactersPerTick = 2
const typingTickMs = 22

function Clicks({ target, active }: { target: number; active: boolean }) {
  return <>{formatCount(useCountUp(target, active, phaseDuration.counting))}</>
}

/**
 * A long URL is typed, an orange sweep crosses it out, and its short link pops
 * in while the click counter runs up; then the next example. It only plays
 * while on screen, and with reduced motion it shows the first example at rest.
 */
type LinkShrinkDemoProps = {
  /**
   * Called when a short link appears, with its code and element, so the page
   * can fly it to its floating twin. The returned function stops the flight.
   */
  onShortLink?: (code: string, element: HTMLElement) => (() => void) | void
}

export function LinkShrinkDemo({ onShortLink }: LinkShrinkDemoProps) {
  const reduced = usePrefersReducedMotion()
  const shortLinkRef = useRef<HTMLDivElement>(null)
  const [ref, inView] = useInView<HTMLDivElement>({ once: false, threshold: 0.4 })
  const [index, setIndex] = useState(0)
  const [typed, setTyped] = useState(0)
  const [phase, setPhase] = useState<Phase>('typing')
  const running = inView && !reduced
  const example = examples[index]

  useEffect(() => {
    if (!running) {
      return
    }

    let timer: number
    if (phase === 'typing') {
      timer =
        typed < example.long.length
          ? window.setTimeout(() => setTyped((count) => Math.min(count + charactersPerTick, example.long.length)), typingTickMs)
          : window.setTimeout(() => setPhase('shrinking'), 250)
    } else {
      timer = window.setTimeout(() => {
        if (phase === 'leaving') {
          setIndex((current) => (current + 1) % examples.length)
          setTyped(0)
        }
        setPhase(nextPhase[phase])
      }, phaseDuration[phase])
    }
    return () => window.clearTimeout(timer)
  }, [running, phase, typed, example])

  const shown = reduced ? { typed: example.long.length, phase: 'holding' as Phase } : { typed, phase }
  const typing = shown.phase === 'typing'
  const swept = !typing
  const revealed = ['revealed', 'counting', 'holding'].includes(shown.phase)
  const counting = shown.phase === 'counting' || shown.phase === 'holding'
  const savedPercent = Math.round((1 - example.short.length / example.long.length) * 100)
  // From the pop-in until the card leaves: long enough for the dots to land.
  const launch = running && revealed

  useEffect(() => {
    const element = shortLinkRef.current
    if (!launch || !element || !onShortLink) {
      return
    }
    const code = example.short.slice(example.short.lastIndexOf('/') + 1)
    return onShortLink(code, element) ?? undefined
  }, [launch, example, onShortLink])

  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="mx-auto mt-10 w-full max-w-4xl rounded-3xl border border-white/10 bg-white/[0.04] p-4 text-left sm:p-6"
    >
      <div className="flex items-center justify-between gap-3">
        <span className="inline-flex items-center gap-2 text-xs font-bold tracking-widest text-navy-300 uppercase">
          <span className="relative flex size-2">
            <span className="absolute inline-flex size-full rounded-full bg-brand opacity-75 motion-safe:animate-ping" />
            <span className="relative inline-flex size-2 rounded-full bg-brand" />
          </span>
          Veja como funciona
        </span>
        <span className="flex gap-1.5">
          {examples.map((item, dot) => (
            <span
              key={item.short}
              className={cn('h-1.5 rounded-full transition-[width,background-color] duration-300', dot === index ? 'w-5 bg-brand' : 'w-1.5 bg-white/25')}
            />
          ))}
        </span>
      </div>

      <div
        className={cn(
          'mt-4 grid items-center gap-3 transition-opacity duration-300 md:grid-cols-[minmax(0,1.15fr)_auto_minmax(0,1fr)] md:gap-4',
          shown.phase === 'leaving' && 'opacity-0',
        )}
      >
        <div className="relative overflow-hidden rounded-xl bg-navy-800/70 px-4 py-3">
          <p className="text-[11px] font-semibold tracking-wider text-navy-300 uppercase">
            Link longo · {example.long.length} caracteres
          </p>
          <p
            className={cn(
              'mt-1 line-clamp-2 min-h-10 font-mono text-[13px] leading-5 break-all text-white/90 decoration-brand decoration-2 transition-opacity duration-500',
              swept && 'line-through opacity-45',
            )}
          >
            {example.long.slice(0, shown.typed)}
            {typing && <span className="ml-0.5 inline-block h-4 w-0.5 translate-y-0.5 bg-brand motion-safe:animate-caret" />}
          </p>
          <span
            className={cn(
              'absolute inset-x-0 bottom-0 h-1 origin-left bg-brand transition-transform duration-700 ease-[cubic-bezier(0.65,0,0.35,1)]',
              swept ? 'scale-x-100' : 'scale-x-0',
            )}
          />
        </div>

        <span
          className={cn(
            'mx-auto inline-flex size-9 rotate-90 items-center justify-center rounded-full bg-brand text-white shadow-sm transition-transform duration-500 md:rotate-0',
            swept ? 'scale-100' : 'scale-75',
          )}
        >
          <ArrowRight className="size-4" />
        </span>

        <div
          ref={shortLinkRef}
          className={cn(
            'flex items-center justify-between gap-3 rounded-xl bg-white px-4 py-3 shadow-raised transition-[opacity,transform] duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)]',
            revealed ? 'translate-y-0 scale-100 opacity-100' : 'translate-y-2 scale-90 opacity-0',
          )}
        >
          <div className="min-w-0">
            <p className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
              Link curto · {example.short.length} caracteres
            </p>
            <p className="mt-0.5 truncate text-base font-extrabold text-action">{example.short}</p>
          </div>
          <div className="shrink-0 text-right">
            <p className="text-xl leading-none font-extrabold text-navy tabular-nums">
              <Clicks key={index} target={example.clicks} active={counting} />
            </p>
            <p className="mt-1 text-[11px] font-medium text-muted-foreground">cliques</p>
          </div>
        </div>
      </div>

      <p className={cn('mt-3 text-sm text-navy-300 transition-opacity duration-500', revealed ? 'opacity-100' : 'opacity-0')}>
        <span className="font-bold text-brand">{savedPercent}% menor</span> e pronto para compartilhar.
      </p>
    </div>
  )
}
