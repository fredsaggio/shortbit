import { BarChart3, ChevronRight, CircleCheck, Globe, Link2, LinkIcon, Lock, RotateCw } from 'lucide-react'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { isAbortError, messageOf } from '../api/client.ts'
import { createLink, listLinks, shortLinkURL } from '../api/links.ts'
import type { CreatedLink, ShortLink, Visibility } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { CopyButton } from '../components/CopyButton.tsx'
import { LinkListSkeleton } from '../components/PageLoader.tsx'
import { TextField } from '../components/TextField.tsx'
import { VisibilityBadge } from '../components/VisibilityBadge.tsx'
import { clicksLabel, formatCount, formatDate } from '../format.ts'
import { clearPendingLink, readPendingLink } from '../pendingLink.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkLinkPassword, checkURL, minPasswordLength } from '../validation.ts'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

const visibilityItemClass =
  'h-11 flex-1 gap-2 rounded-md px-4 text-[15px] font-semibold text-muted-foreground transition-colors hover:bg-surface hover:text-navy data-[state=on]:bg-navy data-[state=on]:text-white data-[state=on]:shadow-sm sm:flex-none'

function CreateLinkCard({ onCreated }: { onCreated: () => void }) {
  const [url, setUrl] = useState(readPendingLink)
  const [visibility, setVisibility] = useState<Visibility>('public')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<CreatedLink | null>(null)
  const fields = useFieldErrors<'url' | 'linkPassword'>()

  useEffect(clearPendingLink, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setCreated(null)
    const valid = fields.check(event.currentTarget, {
      url: checkURL(url),
      linkPassword: visibility === 'private' ? checkLinkPassword(password) : undefined,
    })
    if (!valid) {
      return
    }

    setSubmitting(true)
    try {
      const link =
        visibility === 'private'
          ? await createLink({ url: url.trim(), visibility, password })
          : await createLink({ url: url.trim(), visibility })
      setCreated(link)
      setUrl('')
      setPassword('')
      onCreated()
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section aria-labelledby="create-link-title" className="rounded-3xl border bg-card p-5 shadow-card sm:p-7">
      <h2 id="create-link-title" className="text-xl font-bold">
        Criar link curto
      </h2>
      <form className="mt-5 grid gap-5" onSubmit={handleSubmit} noValidate>
        <TextField
          label="URL de destino"
          name="url"
          type="url"
          inputMode="url"
          placeholder="https://exemplo.com/um-link-bem-longo"
          leading={<Link2 className="size-5" aria-hidden="true" />}
          value={url}
          error={fields.errors.url}
          onChange={(event) => {
            setUrl(event.target.value)
            fields.clear('url')
          }}
          required
        />

        <div className="grid gap-2">
          <span id="visibility-label" className="text-sm font-semibold text-navy">
            Quem pode abrir
          </span>
          <ToggleGroup
            type="single"
            value={visibility}
            onValueChange={(value) => {
              if (value) {
                setVisibility(value as Visibility)
                fields.clear('linkPassword')
              }
            }}
            aria-labelledby="visibility-label"
            className="flex w-full gap-1 rounded-lg border bg-surface p-1 sm:inline-flex sm:w-auto"
          >
            <ToggleGroupItem value="public" className={visibilityItemClass}>
              <Globe className="size-4" aria-hidden="true" />
              Público
            </ToggleGroupItem>
            <ToggleGroupItem value="private" className={visibilityItemClass}>
              <Lock className="size-4" aria-hidden="true" />
              Com senha
            </ToggleGroupItem>
          </ToggleGroup>
        </div>

        {visibility === 'private' && (
          <div className="animate-in fade-in slide-in-from-top-2 duration-200">
            <TextField
              label="Senha do link"
              name="linkPassword"
              type="password"
              autoComplete="new-password"
              hint={`Quem abrir o link precisa digitar essa senha. Mínimo de ${minPasswordLength} caracteres.`}
              value={password}
              error={fields.errors.linkPassword}
              onChange={(event) => {
                setPassword(event.target.value)
                fields.clear('linkPassword')
              }}
              required
            />
          </div>
        )}

        {error && <Alert tone="error">{error}</Alert>}

        <div>
          <Button type="submit" size="lg" loading={submitting}>
            Criar link curto
          </Button>
        </div>
      </form>

      {created && (
        <div
          role="status"
          className="mt-5 flex flex-wrap items-center gap-3 rounded-xl border border-success/25 bg-success-soft p-4 animate-in fade-in zoom-in-95 duration-200"
        >
          <CircleCheck className="size-5 text-success" aria-hidden="true" />
          <a href={created.short_url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate font-bold text-action hover:underline">
            {created.short_url}
          </a>
          <CopyButton text={created.short_url} />
        </div>
      )}
    </section>
  )
}

function LinkItem({ link }: { link: ShortLink }) {
  const shortURL = shortLinkURL(link.short_code)
  const isPrivate = link.visibility === 'private'

  return (
    <li className="group rounded-2xl border bg-card p-5 shadow-card transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:border-navy/20 hover:shadow-raised">
      <div className="flex items-start gap-4">
        <span
          className={cn(
            'hidden size-11 shrink-0 items-center justify-center rounded-xl sm:inline-flex',
            isPrivate ? 'bg-brand-100 text-brand-700' : 'bg-navy-100 text-navy',
          )}
          aria-hidden="true"
        >
          {isPrivate ? <Lock className="size-5" /> : <Globe className="size-5" />}
        </span>

        <div className="min-w-0 flex-1">
          <Link
            to={`/painel/links/${link.short_code}`}
            className="inline-flex max-w-full items-center gap-1 text-lg font-bold text-action underline-offset-4 hover:text-action-hover hover:underline"
          >
            <span className="truncate">{shortURL.replace(/^https?:\/\//, '')}</span>
            <ChevronRight className="size-4 shrink-0 transition-transform duration-200 group-hover:translate-x-0.5" aria-hidden="true" />
          </Link>
          <p className="mt-1 truncate text-[15px] text-muted-foreground" title={link.original_url}>
            {link.original_url}
          </p>
          <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted-foreground">
            <VisibilityBadge visibility={link.visibility} />
            <span>Criado em {formatDate(link.created_at)}</span>
          </div>
        </div>

        <div className="flex shrink-0 flex-col items-end gap-3">
          <div className="text-right">
            <span className="inline-flex items-center gap-1.5 text-2xl leading-none font-extrabold text-navy">
              <BarChart3 className="size-5 text-brand" aria-hidden="true" />
              {formatCount(link.click_count)}
            </span>
            <span className="mt-1 block text-xs font-medium text-muted-foreground">{clicksLabel(link.click_count)}</span>
          </div>
          <CopyButton text={shortURL} size="sm" />
        </div>
      </div>
    </li>
  )
}

type ListState = {
  links: ShortLink[]
  nextCursor: string | null
  status: 'loading' | 'ready' | 'loading-more' | 'error'
  error: string
}

const initialList: ListState = { links: [], nextCursor: null, status: 'loading', error: '' }

export function DashboardPage() {
  const { state: auth } = useAuth()
  const [list, setList] = useState<ListState>(initialList)
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    listLinks(null, controller.signal).then(
      (page) => setList({ links: page.urls, nextCursor: page.next_cursor, status: 'ready', error: '' }),
      (error: unknown) => {
        if (!isAbortError(error)) {
          setList({ ...initialList, status: 'error', error: messageOf(error) })
        }
      },
    )
    return () => controller.abort()
  }, [reloadKey])

  const reload = useCallback(() => {
    setList(initialList)
    setReloadKey((key) => key + 1)
  }, [])

  async function loadMore() {
    if (!list.nextCursor) {
      return
    }
    setList((current) => ({ ...current, status: 'loading-more', error: '' }))
    try {
      const page = await listLinks(list.nextCursor)
      setList((current) => ({
        links: [...current.links, ...page.urls],
        nextCursor: page.next_cursor,
        status: 'ready',
        error: '',
      }))
    } catch (error) {
      setList((current) => ({ ...current, status: 'ready', error: messageOf(error) }))
    }
  }

  const firstName = auth.status === 'authenticated' ? auth.user.full_name?.split(' ')[0] : undefined

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 sm:py-12">
      <header className="mb-8">
        <h1 className="text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">{firstName ? `Olá, ${firstName}!` : 'Seus links'}</h1>
        {auth.status === 'authenticated' && <p className="mt-1 text-muted-foreground">Conectado como {auth.user.email}.</p>}
      </header>

      <div className="grid gap-8">
        <CreateLinkCard onCreated={reload} />

        <section aria-labelledby="links-title">
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 id="links-title" className="text-xl font-bold">
              Seus links
            </h2>
            <Button type="button" variant="ghost" size="sm" onClick={reload} disabled={list.status === 'loading'}>
              <RotateCw className={cn('size-4', list.status === 'loading' && 'animate-spin')} aria-hidden="true" />
              Atualizar
            </Button>
          </div>

          {list.status === 'loading' ? (
            <>
              <LinkListSkeleton />
              <span className="sr-only" role="status">
                Carregando seus links…
              </span>
            </>
          ) : list.status === 'error' ? (
            <div className="grid justify-items-center gap-4 rounded-2xl border bg-card p-8 text-center">
              <Alert tone="error">{list.error}</Alert>
              <Button type="button" variant="outline" onClick={reload}>
                Tentar de novo
              </Button>
            </div>
          ) : list.links.length === 0 ? (
            <div className="grid justify-items-center gap-3 rounded-2xl border border-dashed border-input/50 bg-card px-6 py-12 text-center">
              <span className="inline-flex size-14 items-center justify-center rounded-2xl bg-brand-100" aria-hidden="true">
                <LinkIcon className="size-7 text-brand" />
              </span>
              <p className="text-lg font-bold">Nenhum link ainda</p>
              <p className="max-w-sm text-muted-foreground">Você ainda não criou nenhum link. Cole uma URL acima para começar.</p>
            </div>
          ) : (
            <>
              <ul className="grid gap-3" aria-labelledby="links-title">
                {list.links.map((link) => (
                  <LinkItem key={link.short_code} link={link} />
                ))}
              </ul>
              {list.error && <Alert tone="error" className="mt-4">{list.error}</Alert>}
              {list.nextCursor && (
                <div className="mt-6 flex justify-center">
                  <Button type="button" variant="outline" onClick={loadMore} loading={list.status === 'loading-more'}>
                    Carregar mais
                  </Button>
                </div>
              )}
              <p className="mt-6 text-center text-sm text-muted-foreground">
                Os cliques são contados em segundo plano e podem levar alguns segundos para aparecer.
              </p>
            </>
          )}
        </section>
      </div>
    </div>
  )
}
