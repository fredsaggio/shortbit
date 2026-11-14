import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { PasswordRule } from '../validation.ts'

type PasswordChecklistProps = {
  /** Referenced by the password inputs' aria-describedby. */
  id: string
  rules: PasswordRule[]
}

// Live account password requirements: each dot fills green with a check as
// its rule is met. Screen readers get the state as text, not by color alone.
export function PasswordChecklist({ id, rules }: PasswordChecklistProps) {
  return (
    <ul id={id} aria-label="Requisitos da senha" className="grid gap-1.5 text-sm">
      {rules.map((rule) => (
        <li
          key={rule.label}
          data-state={rule.met ? 'met' : 'pending'}
          className={cn('flex items-center gap-2 transition-colors duration-200', rule.met ? 'text-success' : 'text-muted-foreground')}
        >
          <span
            aria-hidden="true"
            className={cn(
              'inline-flex size-[18px] shrink-0 items-center justify-center rounded-full border-2 transition-[background-color,border-color,transform] duration-200',
              rule.met ? 'scale-110 border-success bg-success' : 'scale-100 border-input/60 bg-transparent',
            )}
          >
            <Check
              strokeWidth={3.5}
              className={cn('size-3 text-white transition-[opacity,transform] duration-200', rule.met ? 'scale-100 opacity-100' : 'scale-50 opacity-0')}
            />
          </span>
          <span>{rule.label}</span>
          <span className="sr-only">{rule.met ? '(cumprido)' : '(pendente)'}</span>
        </li>
      ))}
    </ul>
  )
}
