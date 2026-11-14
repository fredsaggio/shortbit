// Forms use noValidate and these checks instead of the browser's own bubbles,
// so every message has the same look and wording. The API still validates
// everything again.

export type FieldErrors<Field extends string> = Partial<Record<Field, string>>

export const minPasswordLength = 8
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

export function checkNewPassword(value: string): string | undefined {
  if (!value) {
    return 'Crie uma senha.'
  }
  if ([...value].length < minPasswordLength) {
    return `A senha precisa ter pelo menos ${minPasswordLength} caracteres.`
  }
  return undefined
}

export function checkLinkPassword(value: string): string | undefined {
  if (!value) {
    return 'Defina a senha que vai proteger o link.'
  }
  if ([...value].length < minPasswordLength) {
    return `A senha do link precisa ter pelo menos ${minPasswordLength} caracteres.`
  }
  return undefined
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
