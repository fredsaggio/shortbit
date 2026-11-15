import { useCallback, useRef } from 'react'
import { useScrollEffect } from '../../motion.ts'

// A thin orange bar along the bottom of the header that fills as the page
// scrolls. It reads position, not motion, so it stays on with reduced motion.
export function ScrollProgress() {
  const ref = useRef<HTMLDivElement>(null)

  const update = useCallback((scrollY: number) => {
    const scrollable = document.documentElement.scrollHeight - window.innerHeight
    const progress = scrollable > 0 ? Math.min(scrollY / scrollable, 1) : 0
    if (ref.current) {
      ref.current.style.transform = `scaleX(${progress.toFixed(4)})`
    }
  }, [])
  useScrollEffect(update)

  return <div ref={ref} aria-hidden="true" className="absolute inset-x-0 bottom-0 h-0.5 origin-left scale-x-0 bg-brand" />
}
