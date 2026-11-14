import { cn } from '@/lib/utils'

type LogoProps = {
  tone?: 'light' | 'dark'
  className?: string
  hideTextOnSmall?: boolean
}

export function Logo({ tone = 'dark', className, hideTextOnSmall = false }: LogoProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <svg viewBox="0 0 32 32" className="size-8 shrink-0" aria-hidden="true">
        <rect width="32" height="32" rx="9" className="fill-brand" />
        <path d="M13.5 18.5l5-5" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" />
        <path d="M15 11.2l1.6-1.6a4.4 4.4 0 0 1 6.2 6.2L21.2 17.4" fill="none" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" />
        <path d="M17 20.8l-1.6 1.6a4.4 4.4 0 0 1-6.2-6.2l1.6-1.6" fill="none" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" />
      </svg>
      <span
        className={cn(
          'text-xl font-extrabold tracking-tight',
          tone === 'light' ? 'text-white' : 'text-navy',
          hideTextOnSmall && 'max-[400px]:sr-only',
        )}
      >
        short<span className="text-brand">bit</span>
      </span>
    </span>
  )
}
