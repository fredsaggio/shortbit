import type { ButtonHTMLAttributes } from 'react'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost'
  loading?: boolean
  block?: boolean
  small?: boolean
}

export function Button({ variant = 'primary', loading = false, block = false, small = false, className, disabled, children, ...props }: ButtonProps) {
  const classes = ['button', `button--${variant}`, block ? 'button--block' : '', small ? 'button--small' : '', className ?? '']
    .filter(Boolean)
    .join(' ')

  return (
    <button className={classes} disabled={disabled || loading} aria-busy={loading || undefined} {...props}>
      {loading && <span className="spinner" aria-hidden="true" />}
      {children}
    </button>
  )
}
