import { useEffect, useRef, useState, useSyncExternalStore } from 'react'

// Shared motion helpers. Everything animates transform and opacity only, and
// people who ask the system for less motion get the final state at once.

const reducedMotionQuery = '(prefers-reduced-motion: reduce)'

function subscribeReducedMotion(onChange: () => void): () => void {
  if (typeof window.matchMedia !== 'function') {
    return () => {}
  }
  const query = window.matchMedia(reducedMotionQuery)
  query.addEventListener('change', onChange)
  return () => query.removeEventListener('change', onChange)
}

function readReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(reducedMotionQuery).matches
}

export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(subscribeReducedMotion, readReducedMotion, () => false)
}

const canObserve = typeof IntersectionObserver !== 'undefined'

/**
 * Tells whether the element is on screen. With `once`, it stays true after
 * the first sighting, for entrance animations; without it, it follows the
 * element, for loops that should pause off screen. Without
 * IntersectionObserver (old browsers, tests) everything counts as visible.
 */
export function useInView<T extends Element>({ once = true, threshold = 0.2 } = {}) {
  const ref = useRef<T>(null)
  const [inView, setInView] = useState(!canObserve)

  useEffect(() => {
    const node = ref.current
    if (!node || !canObserve) {
      return
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setInView(true)
          if (once) {
            observer.disconnect()
          }
        } else if (!once) {
          setInView(false)
        }
      },
      // Start a little before the element is fully in, as Bitly does.
      { threshold, rootMargin: '0px 0px -8% 0px' },
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [once, threshold])

  return [ref, inView] as const
}

const easeOutCubic = (progress: number) => 1 - (1 - progress) ** 3

/** Counts from 0 to target once `active` turns true. */
export function useCountUp(target: number, active: boolean, durationMs = 1200): number {
  const reduced = usePrefersReducedMotion()
  const [value, setValue] = useState(0)

  useEffect(() => {
    if (!active || reduced) {
      return
    }

    // A ~60 fps timer rather than requestAnimationFrame: same smoothness for
    // a number, and it also runs where frames are not painted.
    let timer = 0
    const start = performance.now()
    const step = () => {
      const progress = Math.min((performance.now() - start) / durationMs, 1)
      setValue(Math.round(target * easeOutCubic(progress)))
      if (progress < 1) {
        timer = window.setTimeout(step, 16)
      }
    }
    timer = window.setTimeout(step, 16)
    return () => window.clearTimeout(timer)
  }, [active, reduced, target, durationMs])

  return reduced ? target : value
}

// One scroll listener for the whole app, read once per frame. Effects get the
// scroll position and write styles straight to the DOM, so scrolling never
// re-renders React.
type ScrollListener = (scrollY: number) => void
const scrollListeners = new Set<ScrollListener>()
let scrollFrame = 0

function emitScroll() {
  scrollFrame = 0
  const y = window.scrollY
  for (const listener of scrollListeners) {
    listener(y)
  }
}

function onScroll() {
  scrollFrame ||= requestAnimationFrame(emitScroll)
}

/** Calls `listener` with window.scrollY on every frame the page scrolls. */
export function useScrollEffect(listener: ScrollListener, enabled = true) {
  useEffect(() => {
    if (!enabled) {
      return
    }
    if (scrollListeners.size === 0) {
      window.addEventListener('scroll', onScroll, { passive: true })
      window.addEventListener('resize', onScroll, { passive: true })
    }
    scrollListeners.add(listener)
    listener(window.scrollY)
    return () => {
      scrollListeners.delete(listener)
      if (scrollListeners.size === 0) {
        window.removeEventListener('scroll', onScroll)
        window.removeEventListener('resize', onScroll)
      }
    }
  }, [listener, enabled])
}

const finePointerQuery = '(hover: hover) and (pointer: fine)'

function subscribeFinePointer(onChange: () => void): () => void {
  if (typeof window.matchMedia !== 'function') {
    return () => {}
  }
  const query = window.matchMedia(finePointerQuery)
  query.addEventListener('change', onChange)
  return () => query.removeEventListener('change', onChange)
}

function readFinePointer(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(finePointerQuery).matches
}

/** True on devices with a mouse or trackpad and motion allowed; false on touch. */
export function usePointerEffects(): boolean {
  const finePointer = useSyncExternalStore(subscribeFinePointer, readFinePointer, () => false)
  const reduced = usePrefersReducedMotion()
  return finePointer && !reduced
}
