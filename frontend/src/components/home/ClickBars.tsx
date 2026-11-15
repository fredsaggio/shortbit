import { cn } from '@/lib/utils'
import { formatCount } from '../../format.ts'
import { useCountUp, useInView } from '../../motion.ts'

const bars = [28, 46, 38, 60, 52, 76, 100]
const exampleClicks = 1284

// A week of clicks that grows bar by bar each time it scrolls into view.
export function ClickBars() {
  const [ref, inView] = useInView<HTMLDivElement>({ once: false, threshold: 0.5 })
  const clicks = useCountUp(exampleClicks, inView, 1400)

  return (
    <div ref={ref} aria-hidden="true" className="mt-6">
      <p className="text-3xl font-extrabold text-navy tabular-nums">
        {formatCount(clicks)} <span className="text-base font-semibold text-muted-foreground">cliques · exemplo</span>
      </p>
      <div className="mt-4 flex h-24 items-end gap-2">
        {bars.map((height, index) => (
          <span
            key={index}
            style={{ height: `${height}%`, transitionDelay: inView ? `${index * 70}ms` : undefined }}
            className={cn(
              'flex-1 origin-bottom rounded-t-md motion-safe:transition-transform motion-safe:duration-700 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)]',
              index === bars.length - 1 ? 'bg-brand' : 'bg-brand/30',
              inView ? 'scale-y-100' : 'motion-safe:scale-y-0',
            )}
          />
        ))}
      </div>
    </div>
  )
}
