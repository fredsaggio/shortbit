import { afterEach, describe, expect, it, vi } from 'vitest'
import { flyLink, isShown } from './linkFlight.ts'

function box(element: HTMLElement, left: number, top: number, width: number, height: number) {
  element.getBoundingClientRect = () => ({ left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}) }) as DOMRect
  return element
}

describe('flyLink', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    // jsdom has no Web Animations API; drop the stand-in.
    delete (HTMLElement.prototype as { animate?: unknown }).animate
    document.body.innerHTML = ''
  })

  it('flies a stream of dots from the short link to the floating link, then cleans up', () => {
    vi.useFakeTimers()
    const layer = box(document.createElement('div'), 0, 0, 1440, 1000)
    const from = box(document.createElement('div'), 780, 800, 360, 64)
    const to = box(document.createElement('span'), 1230, 265, 180, 28)
    document.body.append(layer, from, to)
    const flights: Keyframe[][] = []
    const cancel = vi.fn()
    HTMLElement.prototype.animate = vi.fn(function (keyframes: Keyframe[] | PropertyIndexedKeyframes | null) {
      flights.push(keyframes as Keyframe[])
      return { cancel } as unknown as Animation
    })
    const onArrive = vi.fn()

    flyLink(layer, from, to, onArrive)

    expect(layer.children).toHaveLength(12)
    // Each dot leaves the side of the short link that faces the target and
    // ends on the floating link.
    for (const frames of flights) {
      const first = String(frames[0].transform)
      const last = String(frames[frames.length - 1].transform)
      const [x0] = first.match(/-?\d+(\.\d+)?/g)!.map(Number)
      const [x1, y1] = last.match(/-?\d+(\.\d+)?/g)!.map(Number)
      expect(x0).toBeGreaterThan(1130)
      expect(Math.abs(x1 - 1320)).toBeLessThan(5)
      expect(Math.abs(y1 - 279)).toBeLessThan(5)
    }
    expect(onArrive).not.toHaveBeenCalled()

    vi.advanceTimersByTime(950)
    expect(onArrive).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1000)
    expect(layer.children).toHaveLength(0)
  })

  it('stops at once when canceled', () => {
    vi.useFakeTimers()
    const layer = box(document.createElement('div'), 0, 0, 1440, 1000)
    const cancel = vi.fn()
    HTMLElement.prototype.animate = vi.fn(() => ({ cancel }) as unknown as Animation)
    const onArrive = vi.fn()

    const stop = flyLink(layer, box(document.createElement('div'), 700, 800, 300, 60), box(document.createElement('span'), 20, 700, 180, 28), onArrive)
    stop()
    vi.advanceTimersByTime(3000)

    expect(layer.children).toHaveLength(0)
    expect(cancel).toHaveBeenCalledTimes(12)
    expect(onArrive).not.toHaveBeenCalled()
  })

  it('only marks the arrival where the Web Animations API is missing', () => {
    const layer = box(document.createElement('div'), 0, 0, 1440, 1000)
    Object.defineProperty(layer, 'animate', { value: undefined })
    const onArrive = vi.fn()

    flyLink(layer, box(document.createElement('div'), 0, 0, 10, 10), box(document.createElement('span'), 50, 50, 10, 10), onArrive)

    expect(onArrive).toHaveBeenCalledTimes(1)
    expect(layer.children).toHaveLength(0)
  })

  it('treats a hidden floating link as not shown', () => {
    expect(isShown(box(document.createElement('span'), 0, 0, 0, 0))).toBe(false)
    expect(isShown(box(document.createElement('span'), 10, 10, 180, 28))).toBe(true)
  })
})
