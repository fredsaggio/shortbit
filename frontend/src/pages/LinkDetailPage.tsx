import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError, isAbortError, messageOf } from '../api/client.ts'
import { getLink, shortLinkURL } from '../api/links.ts'
import type { ShortLink } from '../api/types.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { CopyButton } from '../components/CopyButton.tsx'
import { PageLoader } from '../components/PageLoader.tsx'
import { VisibilityBadge } from '../components/VisibilityBadge.tsx'
import { clicksLabel, formatCount, formatDate } from '../format.ts'

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
    <div className="container page">
      <Link to="/painel" className="back-link">
        ← Voltar para meus links
      </Link>

      {state.status === 'loading' && <PageLoader label="Carregando o link…" />}

      {state.status === 'not-found' && (
        <div className="card empty-state">
          <h1>Link não encontrado</h1>
          <p>Esse link não existe ou não pertence à sua conta.</p>
        </div>
      )}

      {state.status === 'error' && (
        <div className="card empty-state">
          <Alert tone="error">{state.error}</Alert>
          <p style={{ marginTop: 16 }}>
            <Button type="button" variant="secondary" onClick={refresh}>
              Tentar de novo
            </Button>
          </p>
        </div>
      )}

      {state.status === 'ready' && (
        <div className="detail">
          <section className="card" aria-labelledby="detail-title">
            <h1 id="detail-title" className="detail__short">
              {shortLinkURL(state.link.short_code).replace(/^https?:\/\//, '')}
            </h1>
            <div className="detail__actions">
              <CopyButton text={shortLinkURL(state.link.short_code)} label="Copiar link" />
              <a className="button button--secondary" href={shortLinkURL(state.link.short_code)} target="_blank" rel="noreferrer">
                Abrir
              </a>
            </div>
            <dl className="detail__facts">
              <div>
                <dt>Destino</dt>
                <dd>
                  <a href={state.link.original_url} target="_blank" rel="noreferrer">
                    {state.link.original_url}
                  </a>
                </dd>
              </div>
              <div>
                <dt>Acesso</dt>
                <dd>
                  <VisibilityBadge visibility={state.link.visibility} />
                </dd>
              </div>
              <div>
                <dt>Criado em</dt>
                <dd>{formatDate(state.link.created_at)}</dd>
              </div>
            </dl>
          </section>

          <aside className="card stat" aria-label="Cliques">
            <p className="stat__value">{formatCount(state.link.click_count)}</p>
            <p className="stat__label">{clicksLabel(state.link.click_count)} no total</p>
            <Button type="button" variant="secondary" onClick={refresh} loading={state.refreshing}>
              Atualizar
            </Button>
            <p className="note" style={{ color: 'inherit', opacity: 0.8 }}>
              Os cliques podem levar alguns segundos para aparecer.
            </p>
          </aside>
        </div>
      )}
    </div>
  )
}
