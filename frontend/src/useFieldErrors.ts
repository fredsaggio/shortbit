import { useCallback, useState } from 'react'
import { focusFirstInvalid, hasErrors, type FieldErrors } from './validation.ts'

export function useFieldErrors<Field extends string>() {
  const [errors, setErrors] = useState<FieldErrors<Field>>({})

  const clear = useCallback((field: Field) => {
    setErrors((current) => (current[field] ? { ...current, [field]: undefined } : current))
  }, [])

  // Shows the errors and focuses the first invalid field; true means valid.
  const check = useCallback((form: HTMLFormElement, next: FieldErrors<Field>): boolean => {
    setErrors(next)
    if (hasErrors(next)) {
      focusFirstInvalid(form, next)
      return false
    }
    return true
  }, [])

  return { errors, clear, check }
}
