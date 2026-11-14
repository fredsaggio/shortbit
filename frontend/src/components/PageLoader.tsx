import { Skeleton } from '@/components/ui/skeleton'

// Skeletons keep the page's shape while loading, instead of a lone spinner.
export function PageLoader({ label = 'Carregando…' }: { label?: string }) {
  return (
    <div role="status" aria-label={label} className="mx-auto w-full max-w-5xl px-4 py-12 sm:px-6">
      <Skeleton className="h-9 w-56 bg-surface-200" />
      <Skeleton className="mt-3 h-5 w-80 max-w-full bg-surface-200" />
      <div className="mt-8 grid gap-4">
        <Skeleton className="h-32 rounded-2xl bg-surface-200" />
        <Skeleton className="h-24 rounded-2xl bg-surface-200" />
        <Skeleton className="h-24 rounded-2xl bg-surface-200" />
      </div>
      <span className="sr-only">{label}</span>
    </div>
  )
}

export function LinkListSkeleton() {
  return (
    <ul className="grid gap-3" aria-hidden="true">
      {[0, 1, 2].map((item) => (
        <li key={item} className="rounded-2xl border bg-card p-5 shadow-card">
          <Skeleton className="h-5 w-48 bg-surface-200" />
          <Skeleton className="mt-3 h-4 w-80 max-w-full bg-surface-200" />
          <Skeleton className="mt-4 h-6 w-40 bg-surface-200" />
        </li>
      ))}
    </ul>
  )
}
