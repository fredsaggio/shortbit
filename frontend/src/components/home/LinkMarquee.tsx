import { cn } from '@/lib/utils'

// A strip of short links that runs sideways without end at the foot of the
// hero, on every screen size. Decorative: hidden from assistive tech, and
// still with reduced motion.
const links = [
  { text: 'shortbit.com.br/Ab3dX9', locked: false },
  { text: 'shortbit.com.br/Qz7Rt2', locked: true },
  { text: 'shortbit.com.br/Lm4Kp8', locked: false },
  { text: 'shortbit.com.br/Tn42Az', locked: false },
  { text: 'shortbit.com.br/Gq3Lm8', locked: true },
  { text: 'shortbit.com.br/Bf26Xq', locked: false },
  { text: 'shortbit.com.br/Pz8Wc1', locked: false },
  { text: 'shortbit.com.br/Hk5Vn7', locked: true },
]

function Row() {
  return (
    <div className="flex shrink-0 gap-3 pr-3">
      {links.map((link) => (
        <span
          key={link.text}
          className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.06] px-3 py-1.5 font-mono text-xs whitespace-nowrap text-navy-300"
        >
          <span className={cn('size-1.5 rounded-full', link.locked ? 'bg-brand' : 'bg-success-soft')} />
          {link.text}
        </span>
      ))}
    </div>
  )
}

export function LinkMarquee() {
  return (
    <div
      aria-hidden="true"
      className="relative mt-12 overflow-hidden [mask-image:linear-gradient(to_right,transparent,black_12%,black_88%,transparent)]"
    >
      <div className="flex w-max motion-safe:animate-marquee">
        <Row />
        <Row />
      </div>
    </div>
  )
}
