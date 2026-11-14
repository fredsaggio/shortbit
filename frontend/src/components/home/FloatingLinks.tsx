import { cn } from '@/lib/utils'

// Short links drifting at the sides of the hero, like Bitly's floating cards.
// Decorative only: hidden from assistive tech, still with reduced motion, and
// shown only from 1360px, where the side margins fit them clear of the cards.
const chips = [
  { text: 'shortbit.com.br/Ab3dX9', position: 'left-[2%] top-[14%]', delay: '0s' },
  { text: 'shortbit.com.br/Qz7Rt2', position: 'left-[3.5%] top-[46%]', delay: '-2.5s', locked: true },
  { text: 'shortbit.com.br/Lm4Kp8', position: 'left-[1.5%] top-[80%]', delay: '-5s' },
  { text: 'shortbit.com.br/Tn42Az', position: 'right-[2%] top-[20%]', delay: '-1.5s' },
  { text: 'shortbit.com.br/Gq3Lm8', position: 'right-[3.5%] top-[54%]', delay: '-4s', locked: true },
  { text: 'shortbit.com.br/Bf26Xq', position: 'right-[1.5%] top-[86%]', delay: '-6s' },
]

export function FloatingLinks() {
  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0 hidden min-[1360px]:block">
      {chips.map((chip, index) => (
        <span
          key={chip.text}
          style={{ animationDelay: chip.delay, transitionDelay: `${index * 120}ms` }}
          className={cn(
            'absolute inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.06] px-2.5 py-1.5 font-mono text-[11px] text-navy-300 shadow-sm motion-safe:animate-float',
            chip.position,
          )}
        >
          <span className={cn('size-1.5 rounded-full', chip.locked ? 'bg-brand' : 'bg-success-soft')} />
          {chip.text}
        </span>
      ))}
    </div>
  )
}
