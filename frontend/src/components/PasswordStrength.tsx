import { cn } from '@/lib/utils'
import { usePasswordStrength } from '../passwordStrength.ts'
import { minPasswordLength } from '../validation.ts'

type PasswordStrengthProps = {
  /** Referenced by the password input's aria-describedby. */
  id: string
  password: string
}

const levels = [
  { label: 'Muito fraca', bar: 'bg-destructive', text: 'text-destructive' },
  { label: 'Fraca', bar: 'bg-brand-700', text: 'text-brand-700' },
  { label: 'Média', bar: 'bg-[#C99A06]', text: 'text-[#7A5C00]' },
  { label: 'Forte', bar: 'bg-success', text: 'text-success' },
  { label: 'Muito forte', bar: 'bg-[#185A2E]', text: 'text-[#185A2E]' },
]
const segments = levels.length

// A five-step strength bar for link passwords. Under the API minimum it says
// so; above it, the zxcvbn score fills the bar. The level is also written out,
// so it never depends on color alone.
export function PasswordStrength({ id, password }: PasswordStrengthProps) {
  const score = usePasswordStrength(password)
  const tooShort = password !== '' && [...password].length < minPasswordLength

  const filled = password === '' ? 0 : tooShort ? 1 : score === null ? 0 : score + 1
  const level = tooShort ? levels[0] : score === null ? null : levels[score]
  const label = password === '' ? null : tooShort ? `Curta demais · mínimo de ${minPasswordLength} caracteres` : level?.label

  return (
    <div id={id} className="grid gap-1.5">
      <div aria-hidden="true" className="grid grid-cols-5 gap-1.5">
        {Array.from({ length: segments }, (_, index) => (
          <span key={index} className="h-1.5 overflow-hidden rounded-full bg-surface-200">
            <span
              className={cn(
                'block h-full origin-left rounded-full transition-transform duration-300 ease-out',
                level?.bar,
                index < filled ? 'scale-x-100' : 'scale-x-0',
              )}
              style={{ transitionDelay: `${index * 40}ms` }}
            />
          </span>
        ))}
      </div>
      <p aria-live="polite" className="min-h-5 text-sm text-muted-foreground">
        {label && (
          <>
            Força da senha: <span className={cn('font-semibold', level?.text)}>{label}</span>
          </>
        )}
      </p>
    </div>
  )
}
