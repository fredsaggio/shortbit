// Bitly-style buttons: 44px tall (WCAG touch target), 8px radius, semibold.
export const buttonTones = {
  primary: 'bg-action text-white shadow-sm hover:bg-action-hover active:bg-action-hover',
  navy: 'bg-navy text-white shadow-sm hover:bg-navy-800',
  outline: 'border-navy/25 bg-card text-navy hover:border-navy hover:bg-surface',
  ghost: 'text-navy hover:bg-navy/5',
  onDark: 'bg-white text-navy hover:bg-surface',
  onDarkOutline: 'border-white/40 bg-transparent text-white hover:border-white hover:bg-white/10',
  // Destructive actions: delete a link, delete the account.
  danger: 'bg-destructive text-white shadow-sm hover:bg-destructive/90',
  dangerGhost: 'text-destructive hover:bg-destructive/5',
} as const

export const buttonSizes = {
  sm: 'h-9 px-3 text-sm',
  md: 'h-11 px-5 text-[15px]',
  lg: 'h-12 px-6 text-base',
} as const
