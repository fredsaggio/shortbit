import { useId, type InputHTMLAttributes } from 'react'

type TextFieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string
  hint?: string
}

export function TextField({ label, hint, id, className, ...props }: TextFieldProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const hintId = hint ? `${inputId}-hint` : undefined

  return (
    <div className={['field', className ?? ''].filter(Boolean).join(' ')}>
      <label className="field__label" htmlFor={inputId}>
        {label}
      </label>
      <input className="field__input" id={inputId} aria-describedby={hintId} {...props} />
      {hint && (
        <p className="field__hint" id={hintId}>
          {hint}
        </p>
      )}
    </div>
  )
}
