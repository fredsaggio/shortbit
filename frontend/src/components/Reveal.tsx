import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { useInView } from '../motion.ts'

type RevealProps = {
  children: ReactNode
  /** Milliseconds to wait after the element comes into view, for staggering. */
  delay?: number
  className?: string
}

// Fades and lifts its content into place the first time it scrolls into view.
// The hidden start state applies only with motion allowed, so with reduced
// motion the content is simply there.
export function Reveal({ children, delay = 0, className }: RevealProps) {
  const [ref, inView] = useInView<HTMLDivElement>()

  return (
    <div
      ref={ref}
      style={inView && delay ? { transitionDelay: `${delay}ms` } : undefined}
      className={cn(
        'motion-safe:transition-[opacity,transform] motion-safe:duration-700 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)]',
        inView ? 'translate-y-0 opacity-100' : 'motion-safe:translate-y-6 motion-safe:opacity-0',
        className,
      )}
    >
      {children}
    </div>
  )
}
