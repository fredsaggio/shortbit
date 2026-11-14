import { Loader2 } from 'lucide-react'
import type { ComponentProps } from 'react'
import { Button as BaseButton } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { buttonSizes, buttonTones } from './buttonStyles.ts'

type ButtonProps = Omit<ComponentProps<typeof BaseButton>, 'variant' | 'size'> & {
  variant?: keyof typeof buttonTones
  size?: 'md' | 'sm' | 'lg'
  loading?: boolean
  block?: boolean
}

export function Button({ variant = 'primary', size = 'md', loading = false, block = false, className, disabled, children, ...props }: ButtonProps) {
  return (
    <BaseButton
      variant="ghost"
      className={cn(
        'gap-2 rounded-md font-semibold transition-[background-color,border-color,color,box-shadow,transform] duration-150',
        'active:translate-y-px disabled:opacity-60',
        buttonSizes[size],
        buttonTones[variant],
        block && 'w-full',
        className,
      )}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
      {children}
    </BaseButton>
  )
}

