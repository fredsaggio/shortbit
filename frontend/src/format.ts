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
