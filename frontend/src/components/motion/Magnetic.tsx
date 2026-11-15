import { useRef, type PointerEvent, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { usePointerEffects } from '../../motion.ts'

type MagneticProps = {
  children: ReactNode
  className?: string
  /** How far the content follows the cursor, in pixels at the edge. */
  strength?: number
}

// Pulls its content a few pixels toward the cursor, for primary calls to action.
export function Magnetic({ children, className, strength = 6 }: MagneticProps) {
  const ref = useRef<HTMLSpanElement>(null)
  const enabled = usePointerEffects()

  function handleMove(event: PointerEvent<HTMLSpanElement>) {
    const node = ref.current
    if (!node || !enabled) {
      return
    }
    const rect = node.getBoundingClientRect()
    const x = ((event.clientX - rect.left) / rect.width - 0.5) * 2
    const y = ((event.clientY - rect.top) / rect.height - 0.5) * 2
    node.style.transform = `translate3d(${(x * strength).toFixed(1)}px, ${(y * strength).toFixed(1)}px, 0)`
  }

  function handleLeave() {
    if (ref.current) {
      ref.current.style.transform = ''
    }
  }

  return (
    <span
      ref={ref}
      onPointerMove={handleMove}
      onPointerLeave={handleLeave}
      className={cn('inline-flex transition-transform duration-200 ease-out', className)}
    >
      {children}
    </span>
  )
}
