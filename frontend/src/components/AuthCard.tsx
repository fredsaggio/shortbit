import type { ReactNode } from 'react'

type AuthCardProps = {
  title: string
  description: ReactNode
  children: ReactNode
  footer?: ReactNode
}

export function AuthCard({ title, description, children, footer }: AuthCardProps) {
  return (
    <section className="flex justify-center px-4 py-12 sm:py-16">
      <div className="w-full max-w-[460px] animate-in fade-in slide-in-from-bottom-2 duration-300">
        <div className="rounded-3xl border bg-card p-6 shadow-card sm:p-9">
          <header>
            <h1 className="text-[28px] leading-tight font-extrabold">{title}</h1>
            <p className="mt-2 text-muted-foreground">{description}</p>
          </header>
          <div className="mt-7 grid gap-5">{children}</div>
        </div>
        {footer && <p className="mt-6 text-center text-[15px] text-muted-foreground">{footer}</p>}
      </div>
    </section>
  )
}

export function OrDivider({ label = 'ou' }: { label?: string }) {
  return (
    <div className="flex items-center gap-3 text-sm text-muted-foreground" role="separator" aria-label={label}>
      <span className="h-px flex-1 bg-border" />
      {label}
      <span className="h-px flex-1 bg-border" />
    </div>
  )
}

export const textLinkClass = 'font-semibold text-action underline-offset-4 transition-colors hover:text-action-hover hover:underline'
