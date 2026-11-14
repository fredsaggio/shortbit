import { CircleAlert } from 'lucide-react'
import { useId, type InputHTMLAttributes, type ReactNode } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

type TextFieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string
  hint?: string
  error?: string
  hideLabel?: boolean
  leading?: ReactNode
  /** Ids of extra descriptions, such as a password checklist. */
  describedBy?: string
}

export function TextField({ label, hint, error, hideLabel = false, leading, describedBy: extraDescribedBy, id, className, ...props }: TextFieldProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const hintId = hint ? `${inputId}-hint` : undefined
  const errorId = error ? `${inputId}-error` : undefined
  const describedBy = [errorId, hintId, extraDescribedBy].filter(Boolean).join(' ') || undefined

  return (
    <div className={cn('grid gap-2', className)}>
      <Label htmlFor={inputId} className={cn('text-sm font-semibold text-navy', hideLabel && 'sr-only')}>
        {label}
      </Label>
      <div className="relative">
        {leading && <span className="pointer-events-none absolute inset-y-0 left-3.5 flex items-center text-muted-foreground">{leading}</span>}
        <Input
          id={inputId}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn(
            'h-12 rounded-md border-input bg-card px-4 text-base text-navy shadow-xs transition-[border-color,box-shadow] duration-150 md:text-base',
            'placeholder:text-muted-foreground/80 hover:border-navy/60',
            'focus-visible:border-brand focus-visible:ring-4 focus-visible:ring-brand/25',
            'aria-invalid:border-destructive aria-invalid:ring-destructive/15',
            leading && 'pl-11',
          )}
          {...props}
        />
      </div>
      {error && (
        <p id={errorId} className="flex items-center gap-1.5 text-sm font-medium text-destructive animate-in fade-in slide-in-from-top-1 duration-150">
          <CircleAlert className="size-4 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
      {hint && (
        <p id={hintId} className="text-sm text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  )
}
