const dateFormat = new Intl.DateTimeFormat('pt-BR', { dateStyle: 'medium', timeStyle: 'short' })
const numberFormat = new Intl.NumberFormat('pt-BR')

export function formatDate(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : dateFormat.format(date)
}

export function formatCount(value: number): string {
  return numberFormat.format(value)
}

export function clicksLabel(value: number): string {
  return value === 1 ? 'clique' : 'cliques'
}

const longDateFormat = new Intl.DateTimeFormat('pt-BR', { dateStyle: 'long' })

export function formatLongDate(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : longDateFormat.format(date)
}

// The greeting uses the first name; accounts without a name fall back to the email.
export function displayName(fullName: string | null, email: string): string {
  return fullName?.split(' ')[0] || email
}
