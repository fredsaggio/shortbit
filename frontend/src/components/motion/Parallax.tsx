import { useCallback, useRef, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { usePrefersReducedMotion, useScrollEffect } from '../../motion.ts'

type ParallaxProps = {
  children?: ReactNode
  className?: string
  /** Fraction of the scroll distance to move: positive lags behind, negative leads. */
  speed: number
  'aria-hidden'?: boolean | 'true'
}

// Moves its content at a different pace from the page as it scrolls.
export function Parallax({ children, className, speed, ...rest }: ParallaxProps) {
  const ref = useRef<HTMLDivElement>(null)
  const reduced = usePrefersReducedMotion()

  const move = useCallback(
    (scrollY: number) => {
      if (ref.current) {
        ref.current.style.transform = `translate3d(0, ${(scrollY * speed).toFixed(1)}px, 0)`
      }
    },
    [speed],
  )
  useScrollEffect(move, !reduced)

  return (
    <div ref={ref} className={cn('will-change-transform', className)} {...rest}>
      {children}
    </div>
  )
}
