import { ArrowLeft, BarChart3, CalendarDays, ExternalLink, Link2Off, RotateCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError, isAbortError, messageOf } from '../api/client.ts'
import { getLink, shortLinkURL } from '../api/links.ts'
import type { ShortLink } from '../api/types.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { buttonSizes, buttonTones } from '../components/buttonStyles.ts'
import { CopyButton } from '../components/CopyButton.tsx'
import { VisibilityBadge } from '../components/VisibilityBadge.tsx'
import { clicksLabel, formatCount, formatDate } from '../format.ts'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

type DetailState =
  | { status: 'loading' }
  | { status: 'ready'; link: ShortLink; refreshing: boolean }
  | { status: 'not-found' }
  | { status: 'error'; error: string }

export function LinkDetailPage() {
  const { code = '' } = useParams()
  const [state, setState] = useState<DetailState>({ status: 'loading' })
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    getLink(code, controller.signal).then(
      (link) => setState({ status: 'ready', link, refreshing: false }),
      (error: unknown) => {
        if (isAbortError(error)) {
          return
        }
        // The API answers 404 both for a missing code and for another
        // user's link, so the page cannot tell them apart either.
        setState(error instanceof ApiError && error.status === 404 ? { status: 'not-found' } : { status: 'error', error: messageOf(error) })
      },
    )
    return () => controller.abort()
  }, [code, reloadKey])

  function refresh() {
    setState((current) => (current.status === 'ready' ? { ...current, refreshing: true } : { status: 'loading' }))
    setReloadKey((key) => key + 1)
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 sm:py-12">
      <Link
        to="/painel"
        className="group mb-6 inline-flex h-10 items-center gap-2 rounded-md px-2 -ml-2 font-semibold text-navy transition-colors hover:bg-navy/5"
      >
        <ArrowLeft className="size-4 transition-transform duration-200 group-hover:-translate-x-0.5" aria-hidden="true" />
        Voltar para meus links
      </Link>

      {state.status === 'loading' && (
        <div className="grid gap-5 lg:grid-cols-[2fr_1fr]" role="status" aria-label="Carregando o link…">
          <Skeleton className="h-72 rounded-3xl bg-surface-200" />
          <Skeleton className="h-72 rounded-3xl bg-surface-200" />
        </div>
      )}

      {state.status === 'not-found' && (
        <div className="grid justify-items-center gap-3 rounded-3xl border bg-card px-6 py-14 text-center">
          <span className="inline-flex size-14 items-center justify-center rounded-2xl bg-surface-200" aria-hidden="true">
            <Link2Off className="size-7 text-navy" />
          </span>
          <h1 className="text-2xl font-extrabold">Link não encontrado</h1>
          <p className="text-muted-foreground">Esse link não existe ou não pertence à sua conta.</p>
        </div>
      )}

      {state.status === 'error' && (
        <div className="grid justify-items-center gap-4 rounded-3xl border bg-card p-8 text-center">
          <Alert tone="error">{state.error}</Alert>
          <Button type="button" variant="outline" onClick={refresh}>
            Tentar de novo
          </Button>
        </div>
      )}

      {state.status === 'ready' && (
        <div className="grid gap-5 lg:grid-cols-[2fr_1fr]">
          <section aria-labelledby="detail-title" className="rounded-3xl border bg-card p-6 shadow-card sm:p-8">
            <VisibilityBadge visibility={state.link.visibility} />
            <h1 id="detail-title" className="mt-4 text-[clamp(1.5rem,3.5vw,2.25rem)] font-extrabold break-all">
              {shortLinkURL(state.link.short_code).replace(/^https?:\/\//, '')}
            </h1>
            <div className="mt-5 flex flex-wrap gap-3">
              <CopyButton text={shortLinkURL(state.link.short_code)} label="Copiar link" variant="navy" />
              <a
                className={cn('inline-flex items-center gap-2 rounded-md border font-semibold transition-colors', buttonSizes.md, buttonTones.outline)}
                href={shortLinkURL(state.link.short_code)}
                target="_blank"
                rel="noreferrer"
              >
                <ExternalLink className="size-4" aria-hidden="true" />
                Abrir
              </a>
            </div>

            <dl className="mt-8 grid gap-5 border-t pt-6">
              <div>
                <dt className="text-sm font-medium text-muted-foreground">Destino</dt>
                <dd className="mt-1 break-all">
                  <a href={state.link.original_url} target="_blank" rel="noreferrer" className="font-semibold text-action underline-offset-4 hover:underline">
                    {state.link.original_url}
                  </a>
                </dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-muted-foreground">Criado em</dt>
                <dd className="mt-1 inline-flex items-center gap-2 font-semibold">
                  <CalendarDays className="size-4 text-muted-foreground" aria-hidden="true" />
                  {formatDate(state.link.created_at)}
                </dd>
              </div>
            </dl>
          </section>

          <aside aria-label="Cliques" className="relative overflow-hidden rounded-3xl bg-navy p-6 text-white shadow-card sm:p-8">
            <div aria-hidden="true" className="pointer-events-none absolute -top-16 -right-16 size-48 rounded-full bg-brand/30 blur-2xl" />
            <div className="relative">
              <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand uppercase">
                <BarChart3 className="size-5" aria-hidden="true" /> Cliques
              </span>
              <p className="mt-4 text-6xl leading-none font-extrabold tracking-tight">{formatCount(state.link.click_count)}</p>
              <p className="mt-2 text-navy-300">{clicksLabel(state.link.click_count)} no total</p>
              <Button type="button" variant="onDarkOutline" className="mt-6" onClick={refresh} loading={state.refreshing}>
                {!state.refreshing && <RotateCw className="size-4" aria-hidden="true" />}
                Atualizar
              </Button>
              <p className="mt-4 text-sm text-navy-300">Os cliques podem levar alguns segundos para aparecer.</p>
            </div>
          </aside>
        </div>
      )}
    </div>
  )
}
