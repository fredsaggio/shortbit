// Dots that carry a freshly shortened link from the demo to its twin among
// the floating links. Plain DOM and the Web Animations API: the dots live for
// about a second, so they never touch React state.

const dotCount = 12
const flightMs = 950
const staggerMs = 45
const samples = 18
const colors = ['#f36600', '#ff8a3d', '#ffffff']

type Point = { x: number; y: number }

function center(rect: DOMRect, origin: DOMRect): Point {
  return { x: rect.left + rect.width / 2 - origin.left, y: rect.top + rect.height / 2 - origin.top }
}

// A point on the quadratic curve from start to end bent through control.
function onCurve(start: Point, control: Point, end: Point, t: number): Point {
  const u = 1 - t
  return {
    x: u * u * start.x + 2 * u * t * control.x + t * t * end.x,
    y: u * u * start.y + 2 * u * t * control.y + t * t * end.y,
  }
}

/** Whether the element is laid out on screen (the floating links hide on narrow screens). */
export function isShown(element: Element): boolean {
  const rect = element.getBoundingClientRect()
  return rect.width > 0 && rect.height > 0
}

/**
 * Flies dots inside layer from the element from to the element to, and calls
 * onArrive when the first ones land. Returns a function that stops them.
 * Without the Web Animations API (old browsers, tests), it only calls onArrive.
 */
export function flyLink(layer: HTMLElement, from: HTMLElement, to: HTMLElement, onArrive: () => void): () => void {
  const origin = layer.getBoundingClientRect()
  const fromRect = from.getBoundingClientRect()
  const toRect = to.getBoundingClientRect()
  const end = center(toRect, origin)
  // Leave from the side of the short link that faces the target.
  const start = {
    x: (end.x < center(fromRect, origin).x ? fromRect.left : fromRect.right) - origin.left,
    y: fromRect.top + fromRect.height / 2 - origin.top,
  }

  if (typeof layer.animate !== 'function') {
    onArrive()
    return () => {}
  }

  const dots: HTMLSpanElement[] = []
  const animations: Animation[] = []
  for (let i = 0; i < dotCount; i += 1) {
    const size = 4 + (i % 3) * 1.5
    const color = colors[i % colors.length]
    const dot = document.createElement('span')
    dot.style.cssText = `position:absolute;left:0;top:0;width:${size}px;height:${size}px;border-radius:9999px;background:${color};box-shadow:0 0 10px ${color};pointer-events:none;opacity:0`
    layer.append(dot)
    dots.push(dot)

    // Each dot bends a little differently, so they travel as a loose stream.
    const spread = (i / (dotCount - 1) - 0.5) * 140
    const control = { x: (start.x + end.x) / 2 + spread * 0.4, y: Math.min(start.y, end.y) - 160 + spread }
    const keyframes: Keyframe[] = []
    for (let s = 0; s <= samples; s += 1) {
      const t = s / samples
      const point = onCurve(start, control, end, t)
      keyframes.push({
        transform: `translate(${point.x - size / 2}px, ${point.y - size / 2}px) scale(${1 - t * 0.45})`,
        opacity: t < 0.08 ? t / 0.08 : t > 0.9 ? (1 - t) / 0.1 : 1,
      })
    }
    animations.push(
      dot.animate(keyframes, {
        duration: flightMs,
        delay: i * staggerMs,
        easing: 'cubic-bezier(0.45, 0, 0.2, 1)',
        fill: 'forwards',
      }),
    )
  }

  const arrival = window.setTimeout(onArrive, flightMs)
  const cleanup = window.setTimeout(() => dots.forEach((dot) => dot.remove()), flightMs + dotCount * staggerMs + 50)

  return () => {
    window.clearTimeout(arrival)
    window.clearTimeout(cleanup)
    animations.forEach((animation) => animation.cancel())
    dots.forEach((dot) => dot.remove())
  }
}
