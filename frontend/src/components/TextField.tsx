import { useId, type InputHTMLAttributes } from 'react'

type TextFieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string
  hint?: string
  error?: string
}

export function TextField({ label, hint, error, id, className, ...props }: TextFieldProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const hintId = hint ? `${inputId}-hint` : undefined
  const errorId = error ? `${inputId}-error` : undefined
  const describedBy = [errorId, hintId].filter(Boolean).join(' ') || undefined

  return (
    <div className={['field', error ? 'field--invalid' : '', className ?? ''].filter(Boolean).join(' ')}>
      <label className="field__label" htmlFor={inputId}>
        {label}
      </label>
      <input className="field__input" id={inputId} aria-invalid={error ? true : undefined} aria-describedby={describedBy} {...props} />
      {error && (
        <p className="field__error" id={errorId}>
          <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <circle cx="8" cy="8" r="7" fill="currentColor" />
            <path d="M8 4.5v4.2M8 11.2v.1" stroke="#fff" strokeWidth="1.8" strokeLinecap="round" />
          </svg>
          {error}
        </p>
      )}
      {hint && (
        <p className="field__hint" id={hintId}>
          {hint}
        </p>
      )}
    </div>
  )
}
