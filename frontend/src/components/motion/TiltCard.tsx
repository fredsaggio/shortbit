import { useRef, type PointerEvent, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { usePointerEffects } from '../../motion.ts'

type TiltCardProps = {
  children: ReactNode
  className?: string
  /** Largest tilt in degrees. */
  maxTilt?: number
  /** Spotlight color under the cursor. */
  glow?: 'light' | 'dark'
}

// Tilts its content slightly toward the cursor and lights the spot under it.
// Only with a mouse or trackpad and motion allowed; otherwise it is a plain box.
export function TiltCard({ children, className, maxTilt = 5, glow = 'light' }: TiltCardProps) {
  const ref = useRef<HTMLDivElement>(null)
  const enabled = usePointerEffects()

  function handleMove(event: PointerEvent<HTMLDivElement>) {
    const node = ref.current
    if (!node || !enabled) {
      return
    }
    const rect = node.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rect.width
    const y = (event.clientY - rect.top) / rect.height
    node.style.setProperty('--mx', `${(x * 100).toFixed(1)}%`)
    node.style.setProperty('--my', `${(y * 100).toFixed(1)}%`)
    node.style.transform = `perspective(900px) rotateX(${((0.5 - y) * maxTilt).toFixed(2)}deg) rotateY(${((x - 0.5) * maxTilt).toFixed(2)}deg)`
  }

  function handleLeave() {
    if (ref.current) {
      ref.current.style.transform = ''
    }
  }

  return (
    <div
      ref={ref}
      onPointerMove={handleMove}
      onPointerLeave={handleLeave}
      className={cn('group/tilt relative h-full transition-transform duration-300 ease-out', className)}
    >
      {children}
      {enabled && (
        <span
          aria-hidden="true"
          className={cn(
            'pointer-events-none absolute inset-0 rounded-[inherit] opacity-0 transition-opacity duration-300 group-hover/tilt:opacity-100',
            glow === 'light'
              ? '[background:radial-gradient(420px_circle_at_var(--mx)_var(--my),rgb(243_102_0/0.10),transparent_60%)]'
              : '[background:radial-gradient(420px_circle_at_var(--mx)_var(--my),rgb(255_255_255/0.10),transparent_60%)]',
          )}
        />
      )}
    </div>
  )
}
