import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { isAbortError, messageOf } from '../api/client.ts'
import { createLink, listLinks, shortLinkURL } from '../api/links.ts'
import type { CreatedLink, ShortLink, Visibility } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { CopyButton } from '../components/CopyButton.tsx'
import { PageLoader } from '../components/PageLoader.tsx'
import { TextField } from '../components/TextField.tsx'
import { VisibilityBadge } from '../components/VisibilityBadge.tsx'
import { clicksLabel, formatCount, formatDate } from '../format.ts'
import { clearPendingLink, readPendingLink } from '../pendingLink.ts'

const minLinkPasswordLength = 8

function CreateLinkCard({ onCreated }: { onCreated: () => void }) {
  const [url, setUrl] = useState(readPendingLink)
  const [visibility, setVisibility] = useState<Visibility>('public')

  useEffect(clearPendingLink, [])
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<CreatedLink | null>(null)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setCreated(null)
    setSubmitting(true)
    try {
      const link =
        visibility === 'private'
          ? await createLink({ url, visibility, password })
          : await createLink({ url, visibility })
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
    <section className="card" aria-labelledby="create-link-title">
      <h2 id="create-link-title">Novo link</h2>
      <form className="form" onSubmit={handleSubmit}>
        <div className="create-form__row">
          <TextField
            label="URL de destino"
            type="url"
            inputMode="url"
            placeholder="https://exemplo.com/um-link-bem-longo"
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            required
          />
          <fieldset className="segmented" aria-label="Acesso">
            <label>
              <input
                type="radio"
                name="visibility"
                value="public"
                checked={visibility === 'public'}
                onChange={() => setVisibility('public')}
              />
              <span>Público</span>
            </label>
            <label>
              <input
                type="radio"
                name="visibility"
                value="private"
                checked={visibility === 'private'}
                onChange={() => setVisibility('private')}
              />
              <span>Com senha</span>
            </label>
          </fieldset>
        </div>

        {visibility === 'private' && (
          <TextField
            label="Senha do link"
            type="password"
            autoComplete="new-password"
            minLength={minLinkPasswordLength}
            hint={`Quem abrir o link precisa digitar essa senha. Mínimo de ${minLinkPasswordLength} caracteres.`}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        )}

        {error && <Alert tone="error">{error}</Alert>}

        <div>
          <Button type="submit" loading={submitting}>
            Criar link curto
          </Button>
        </div>
      </form>

      {created && (
        <div className="created-link" style={{ marginTop: 16 }} role="status">
          <a className="created-link__url" href={created.short_url} target="_blank" rel="noreferrer">
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

  return (
    <li className="card link-item">
      <div className="link-item__main">
        <Link to={`/painel/links/${link.short_code}`} className="link-item__short">
          {shortURL.replace(/^https?:\/\//, '')}
        </Link>
        <p className="link-item__original" title={link.original_url}>
          {link.original_url}
        </p>
      </div>
      <div className="clicks">
        <span className="clicks__value">{formatCount(link.click_count)}</span>
        <span className="clicks__label">{clicksLabel(link.click_count)}</span>
      </div>
      <div className="link-item__meta">
        <VisibilityBadge visibility={link.visibility} />
        <span>Criado em {formatDate(link.created_at)}</span>
        <CopyButton text={shortURL} />
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

  return (
    <div className="container page">
      <header className="page__header">
        <h1>Meus links</h1>
        {auth.status === 'authenticated' && <p>Conectado como {auth.user.email}</p>}
      </header>

      <div className="dashboard">
        <CreateLinkCard onCreated={reload} />

        <section aria-labelledby="links-title">
          <div className="row-between">
            <h2 id="links-title">Seus links</h2>
            <Button type="button" variant="ghost" onClick={reload} disabled={list.status === 'loading'}>
              Atualizar
            </Button>
          </div>

          {list.status === 'loading' ? (
            <PageLoader label="Carregando seus links…" />
          ) : list.status === 'error' ? (
            <div className="card empty-state">
              <Alert tone="error">{list.error}</Alert>
              <p style={{ marginTop: 16 }}>
                <Button type="button" variant="secondary" onClick={reload}>
                  Tentar de novo
                </Button>
              </p>
            </div>
          ) : list.links.length === 0 ? (
            <div className="card empty-state">
              <p>Você ainda não criou nenhum link. Cole uma URL acima para começar.</p>
            </div>
          ) : (
            <>
              <ul className="links">
                {list.links.map((link) => (
                  <LinkItem key={link.short_code} link={link} />
                ))}
              </ul>
              {list.error && <Alert tone="error">{list.error}</Alert>}
              {list.nextCursor && (
                <div className="load-more">
                  <Button type="button" variant="secondary" onClick={loadMore} loading={list.status === 'loading-more'}>
                    Carregar mais
                  </Button>
                </div>
              )}
              <p className="note">Os cliques são contados em segundo plano e podem levar alguns segundos para aparecer.</p>
            </>
          )}
        </section>
      </div>
    </div>
  )
}
