import { BarChart3, ChevronRight, CircleCheck, Globe, Link2, LinkIcon, Lock, RotateCw } from 'lucide-react'
import { useCallback, useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Link } from 'react-router'
import { isAbortError, messageOf } from '../api/client.ts'
import { createLink, listLinks, shortLinkURL } from '../api/links.ts'
import type { CreatedLink, ShortLink, Visibility } from '../api/types.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { CopyButton } from '../components/CopyButton.tsx'
import { LinkListSkeleton } from '../components/PageLoader.tsx'
import { PasswordChecklist } from '../components/PasswordChecklist.tsx'
import { TextField } from '../components/TextField.tsx'
import { VisibilityBadge } from '../components/VisibilityBadge.tsx'
import { VisibilityToggle } from '../components/VisibilityToggle.tsx'
import { clicksLabel, formatCount, formatDate } from '../format.ts'
import { clearPendingLink, readPendingLink } from '../pendingLink.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkLinkPassword, checkURL, passwordRules } from '../validation.ts'
import { cn } from '@/lib/utils'

function CreateLinkCard({ onCreated }: { onCreated: () => void }) {
  // A link asked for on the home page before signing in is created as soon
  // as the dashboard opens, so the visitor lands on a link ready to copy. A
  // private one whose password was lost to a page load (the Google sign-in)
  // is never created public: the form comes back filled, asking for it.
  const [pending] = useState(readPendingLink)
  const canAutoCreate = pending !== null && (pending.visibility === 'public' || Boolean(pending.password))
  const needsPassword = pending !== null && !canAutoCreate
  const autoCreateStarted = useRef(false)
  const [url, setUrl] = useState(needsPassword ? pending.url : '')
  const [visibility, setVisibility] = useState<Visibility>(needsPassword ? 'private' : 'public')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(canAutoCreate)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState(
    needsPassword ? 'Digite de novo a senha do link. Por segurança, ela não fica salva no navegador.' : '',
  )
  const [created, setCreated] = useState<CreatedLink | null>(null)
  const fields = useFieldErrors<'url' | 'linkPassword'>()
  const checklistId = useId()

  useEffect(() => {
    // React runs effects twice in development; the ref keeps it to one link.
    if (!pending || autoCreateStarted.current) {
      return
    }
    autoCreateStarted.current = true
    clearPendingLink()
    if (!canAutoCreate) {
      return
    }

    const request =
      pending.visibility === 'private' && pending.password
        ? createLink({ url: pending.url, visibility: 'private', password: pending.password })
        : createLink({ url: pending.url, visibility: 'public' })
    request
      .then(
        (link) => {
          setCreated(link)
          toast.success('Seu link curto está pronto', { description: link.short_url })
          onCreated()
        },
        (createError: unknown) => {
          setUrl(pending.url)
          setVisibility(pending.visibility)
          setPassword(pending.password ?? '')
          setError(messageOf(createError))
        },
      )
      .finally(() => setSubmitting(false))
  }, [pending, canAutoCreate, onCreated])

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
      setNotice('')
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
      <form className="mt-5 grid grid-cols-[minmax(0,1fr)] gap-5" onSubmit={handleSubmit} noValidate>
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

        <VisibilityToggle
          value={visibility}
          onChange={(value) => {
            setVisibility(value)
            fields.clear('linkPassword')
          }}
        />

        {visibility === 'private' && (
          <div className="grid gap-3 animate-in fade-in slide-in-from-top-2 duration-200">
            {notice && <Alert tone="info">{notice}</Alert>}
            <TextField
              autoFocus={needsPassword}
              label="Senha do link"
              name="linkPassword"
              type="password"
              autoComplete="new-password"
              hint="Quem abrir o link precisa digitar essa senha."
              describedBy={checklistId}
              value={password}
              error={fields.errors.linkPassword}
              onChange={(event) => {
                setPassword(event.target.value)
                fields.clear('linkPassword')
              }}
              required
            />
            <PasswordChecklist id={checklistId} rules={passwordRules({ password })} />
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
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
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

        <div className="flex shrink-0 items-center justify-between gap-3 border-t pt-4 sm:flex-col sm:items-end sm:border-t-0 sm:pt-0">
          <div className="flex items-baseline gap-1.5 sm:block sm:text-right">
            <span className="inline-flex items-center gap-1.5 text-2xl leading-none font-extrabold text-navy">
              <BarChart3 className="size-5 text-brand" aria-hidden="true" />
              {formatCount(link.click_count)}
            </span>
            <span className="text-xs font-medium text-muted-foreground sm:mt-1 sm:block">{clicksLabel(link.click_count)}</span>
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

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 sm:py-12">
      <header className="mb-8">
        <h1 className="text-[clamp(1.875rem,4vw,2.5rem)] leading-tight font-extrabold">
          Link curto, alcance <span className="text-brand-700">longo</span>.
        </h1>
        <p className="mt-2 text-muted-foreground">Cole, encurte e veja quem clicou.</p>
      </header>

      <div className="grid grid-cols-[minmax(0,1fr)] gap-8">
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
              <ul className="grid grid-cols-[minmax(0,1fr)] gap-3" aria-labelledby="links-title">
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
