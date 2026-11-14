import { cn } from '@/lib/utils'

type LogoProps = {
  tone?: 'light' | 'dark'
  className?: string
  hideTextOnSmall?: boolean
}

export function Logo({ tone = 'dark', className, hideTextOnSmall = false }: LogoProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      {/* A padlock with an "s": shortbit, and the password-protected links
          that set it apart. Same drawing as public/favicon.svg. */}
      <svg viewBox="0 0 32 32" className="size-8 shrink-0" aria-hidden="true">
        <rect width="32" height="32" rx="8" className="fill-brand" />
        <path d="M10.8 14V11a5.2 5.2 0 0 1 10.4 0v3" fill="none" stroke="#fff" strokeWidth="3" strokeLinecap="round" />
        <rect x="6.5" y="13" width="19" height="14" rx="3.8" fill="#fff" />
        <path
          d="M19 16.9c-.8-.7-1.8-1-2.9-1-1.6 0-2.6.8-2.6 1.9 0 2.7 5.6 1.5 5.6 4.3 0 1.2-1.2 2-2.9 2-1.2 0-2.3-.4-3.1-1.2"
          fill="none"
          strokeWidth="2.4"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="stroke-brand"
        />
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
