import { CircleAlert, CircleCheck, Info } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

const tones = {
  error: { className: 'border-destructive/25 bg-destructive/5 text-destructive', Icon: CircleAlert },
  success: { className: 'border-success/25 bg-success-soft text-success', Icon: CircleCheck },
  info: { className: 'border-action/20 bg-navy-100 text-navy', Icon: Info },
}

type AlertProps = {
  tone: keyof typeof tones
  children: ReactNode
  className?: string
}

// Only errors use role="alert"; success and info are polite status updates.
export function Alert({ tone, children, className }: AlertProps) {
  const { className: toneClass, Icon } = tones[tone]

  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={cn(
        'flex items-start gap-3 rounded-md border px-4 py-3 text-sm font-medium animate-in fade-in slide-in-from-top-1 duration-200',
        toneClass,
        className,
      )}
    >
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0">{children}</div>
    </div>
  )
}
