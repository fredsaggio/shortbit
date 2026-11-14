import { cn } from '@/lib/utils'

type LogoProps = {
  tone?: 'light' | 'dark'
  className?: string
}

// The wordmark alone: "short" in the surface color and "bit" in orange, set in
// Nunito for its rounded terminals.
export function Logo({ tone = 'dark', className }: LogoProps) {
  return (
    <span
      className={cn(
        'inline-flex items-baseline font-logo text-[26px] leading-none font-[850] tracking-[-0.02em]',
        tone === 'light' ? 'text-white' : 'text-navy',
        className,
      )}
    >
      short<span className="text-brand">bit</span>
    </span>
  )
}
