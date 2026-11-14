import { cn } from '@/lib/utils'

type LogoProps = {
  tone?: 'light' | 'dark'
  className?: string
}

// The wordmark alone: "short" in the surface color and "bit" in orange, set in
// Sora for its wide, geometric letters.
export function Logo({ tone = 'dark', className }: LogoProps) {
  return (
    <span
      className={cn(
        'inline-flex items-baseline font-logo text-[25px] leading-none font-bold tracking-[-0.04em]',
        tone === 'light' ? 'text-white' : 'text-navy',
        className,
      )}
    >
      short<span className="text-brand">bit</span>
    </span>
  )
}
