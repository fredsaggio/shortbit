import { cn } from '@/lib/utils'

// Short links drifting at the sides of the hero, like Bitly's floating cards.
// Decorative only: hidden from assistive tech, still with reduced motion, and
// shown only from 1360px, where the side margins fit them clear of the cards.
// Tn42Az, Ap7Rs2 and Bf26Xq are the demo's examples: a link shortened there
// flies to its twin here.
const chips = [
  { code: 'Ab3dX9', position: 'left-[2%] top-[14%]', delay: '0s' },
  { code: 'Qz7Rt2', position: 'left-[3.5%] top-[46%]', delay: '-2.5s', locked: true },
  { code: 'Ap7Rs2', position: 'left-[1.5%] top-[68%]', delay: '-5s' },
  { code: 'Tn42Az', position: 'right-[2%] top-[20%]', delay: '-1.5s' },
  { code: 'Gq3Lm8', position: 'right-[3.5%] top-[54%]', delay: '-4s', locked: true },
  { code: 'Bf26Xq', position: 'right-[1.5%] top-[74%]', delay: '-6s' },
]

type FloatingLinksProps = {
  /** Gives the page each chip's element, by short code, to aim the demo's dots. */
  registerChip?: (code: string, element: HTMLElement | null) => void
  /** The chip that just received a link; key restarts its glow. */
  arrival?: { code: string; key: number } | null
}

export function FloatingLinks({ registerChip, arrival }: FloatingLinksProps) {
  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0 hidden min-[1360px]:block">
      {chips.map((chip) => {
        const arrived = arrival?.code === chip.code
        return (
          <span
            key={chip.code}
            ref={(element) => registerChip?.(chip.code, element)}
            style={{ animationDelay: chip.delay }}
            className={cn(
              'absolute inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1.5 font-mono text-[11px] shadow-sm transition-colors duration-700 motion-safe:animate-float',
              arrived ? 'border-brand/60 bg-brand/15 text-white' : 'border-white/10 bg-white/[0.06] text-navy-300',
              chip.position,
            )}
          >
            {/* Keyed by the arrival, so each link that lands replays the glow. */}
            {arrived && <span key={arrival.key} className="absolute inset-0 rounded-full bg-brand/45 motion-safe:animate-arrive" />}
            <span className={cn('relative size-1.5 rounded-full', chip.locked ? 'bg-brand' : 'bg-success-soft')} />
            <span className="relative">shortbit.com.br/{chip.code}</span>
          </span>
        )
      })}
    </div>
  )
}
