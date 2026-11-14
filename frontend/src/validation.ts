// Forms use noValidate and these checks instead of the browser's own bubbles,
// so every message has the same look and wording. The API still validates
// everything again.

export type FieldErrors<Field extends string> = Partial<Record<Field, string>>

// Same limits as the API (services/user_service.go and url_service.go). There
// are no composition rules on purpose: NIST SP 800-63B says verifiers shall not
// impose them, and length is what makes a password strong.
export const minPasswordLength = 8
export const maxPasswordBytes = 1024
// OWASP considers passwords under 15 characters weak without MFA, so the
// checklist recommends it, without requiring it.
export const recommendedPasswordLength = 15
export const maxFullNameLength = 100

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function checkEmail(value: string): string | undefined {
  if (!value.trim()) {
    return 'Informe seu email.'
  }
  if (!emailPattern.test(value.trim())) {
    return 'Digite um email válido, como nome@exemplo.com.'
  }
  return undefined
}

export function checkRequired(value: string, message: string): string | undefined {
  return value.trim() ? undefined : message
}

const characterCount = (value: string) => [...value].length
const byteCount = (value: string) => new TextEncoder().encode(value).length

export function checkNewPassword(value: string): string | undefined {
  if (!value) {
    return 'Crie uma senha.'
  }
  if (characterCount(value) < minPasswordLength) {
    return `A senha precisa ter pelo menos ${minPasswordLength} caracteres.`
  }
  if (byteCount(value) > maxPasswordBytes) {
    return 'A senha está longa demais. Use uma mais curta.'
  }
  return undefined
}

export function checkLinkPassword(value: string): string | undefined {
  if (!value) {
    return 'Defina a senha que vai proteger o link.'
  }
  if (characterCount(value) < minPasswordLength) {
    return `A senha do link precisa ter pelo menos ${minPasswordLength} caracteres.`
  }
  if (byteCount(value) > maxPasswordBytes) {
    return 'A senha do link está longa demais. Use uma mais curta.'
  }
  return undefined
}

export type PasswordRule = {
  label: string
  met: boolean
  /** Recommended, not required: never blocks the form. */
  optional?: boolean
}

type PasswordRuleInput = {
  password: string
  /** Adds "the two match" when the form has a confirmation field. */
  confirmation?: string
  /** Adds "different from the current one" when changing a password. */
  current?: string
}

// The live checklist under password fields: only what the API enforces, plus
// the recommended length. The byte limit shows up only once it is broken.
export function passwordRules({ password, confirmation, current }: PasswordRuleInput): PasswordRule[] {
  const rules: PasswordRule[] = [{ label: `Pelo menos ${minPasswordLength} caracteres`, met: characterCount(password) >= minPasswordLength }]

  if (byteCount(password) > maxPasswordBytes) {
    rules.push({ label: 'Não muito longa (o limite foi ultrapassado)', met: false })
  }
  if (current !== undefined) {
    rules.push({ label: 'Diferente da senha atual', met: password !== '' && password !== current })
  }
  if (confirmation !== undefined) {
    rules.push({ label: 'As duas senhas são iguais', met: confirmation !== '' && confirmation === password })
  }

  rules.push({
    label: `${recommendedPasswordLength} ou mais caracteres`,
    met: characterCount(password) >= recommendedPasswordLength,
    optional: true,
  })

  return rules
}

export function checkConfirmation(password: string, confirmation: string): string | undefined {
  if (!confirmation) {
    return 'Repita a senha para confirmar.'
  }
  return password === confirmation ? undefined : 'As senhas não coincidem.'
}

export function checkFullName(value: string): string | undefined {
  const name = value.trim()
  if (!name) {
    return 'Informe seu nome completo.'
  }
  if ([...name].length > maxFullNameLength) {
    return `O nome pode ter até ${maxFullNameLength} caracteres.`
  }
  return undefined
}

export function checkURL(value: string): string | undefined {
  const text = value.trim()
  if (!text) {
    return 'Cole a URL que você quer encurtar.'
  }
  try {
    const url = new URL(text)
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return undefined
    }
  } catch {
    // Falls through to the message below.
  }
  return 'Digite uma URL completa, começando com http:// ou https://.'
}

export function checkCode(value: string, digits: number): string | undefined {
  if (!value) {
    return 'Digite o código que chegou no seu email.'
  }
  return new RegExp(`^\\d{${digits}}$`).test(value) ? undefined : `O código tem ${digits} dígitos.`
}

export function hasErrors<Field extends string>(errors: FieldErrors<Field>): boolean {
  return Object.values(errors).some(Boolean)
}

// Moves keyboard focus to the first field with an error, in form order.
export function focusFirstInvalid(form: HTMLFormElement, errors: FieldErrors<string>): void {
  for (const element of Array.from(form.elements)) {
    const name = (element as HTMLInputElement).name
    if (name && errors[name]) {
      ;(element as HTMLElement).focus()
      return
    }
  }
}
