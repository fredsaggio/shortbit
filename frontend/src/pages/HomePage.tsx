import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { messageOf } from '../api/client.ts'
import { createLink } from '../api/links.ts'
import type { CreatedLink } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { CopyButton } from '../components/CopyButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { savePendingLink } from '../pendingLink.ts'

const features = [
  {
    title: 'Público ou com senha',
    text: 'Compartilhe um link aberto para todo mundo ou proteja o destino com uma senha só sua.',
    icon: (
      <path d="M7 11V8a5 5 0 0 1 10 0v3M6 11h12v9H6z" fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
    ),
  },
  {
    title: 'Cliques contados',
    text: 'Cada acesso é registrado. Veja no painel quantas vezes cada link foi aberto.',
    icon: <path d="M5 19V11M12 19V5M19 19v-6" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" />,
  },
  {
    title: 'Rápido sob pico',
    text: 'Links muito acessados saem da memória, e a contagem roda em segundo plano para o redirect não esperar.',
    icon: <path d="M13 3L5 14h6l-1 7 8-11h-6l1-7z" fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />,
  },
]

export function HomePage() {
  const { state } = useAuth()
  const navigate = useNavigate()
  const [url, setUrl] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<CreatedLink | null>(null)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setCreated(null)

    if (state.status !== 'authenticated') {
      savePendingLink(url)
      navigate('/cadastro', { state: { message: 'Crie sua conta grátis para encurtar o seu link. Ele já vai estar no painel.' } })
      return
    }

    setSubmitting(true)
    try {
      setCreated(await createLink({ url, visibility: 'public' }))
      setUrl('')
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <section className="hero">
        <div className="container hero__inner">
          <div>
            <span className="hero__eyebrow">Encurtador de links</span>
            <h1 className="hero__title">
              Links curtos, <em>cliques contados</em>.
            </h1>
            <p className="hero__lead">
              Transforme URLs enormes em links fáceis de compartilhar. Deixe abertos ou proteja com senha e acompanhe quantas
              pessoas clicaram.
            </p>
          </div>

          <div>
            <form className="shorten" onSubmit={handleSubmit}>
              <TextField
                label="URL para encurtar"
                type="url"
                inputMode="url"
                placeholder="Cole aqui um link longo, como https://exemplo.com/artigo"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                required
              />
              <Button type="submit" loading={submitting}>
                Encurtar
              </Button>
            </form>
            {error && <Alert tone="error">{error}</Alert>}
            {created && (
              <div className="created-link" style={{ marginTop: 12 }}>
                <a className="created-link__url" href={created.short_url} target="_blank" rel="noreferrer">
                  {created.short_url}
                </a>
                <CopyButton text={created.short_url} />
              </div>
            )}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <h2 className="section__title">Tudo o que um link curto precisa</h2>
          <p className="section__lead">Sem planos complicados: crie a conta e comece a encurtar.</p>
          <div className="features">
            {features.map((feature) => (
              <article className="card feature" key={feature.title}>
                <span className="feature__icon">
                  <svg viewBox="0 0 24 24" width="24" height="24" aria-hidden="true">
                    {feature.icon}
                  </svg>
                </span>
                <h3>{feature.title}</h3>
                <p>{feature.text}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="section" style={{ paddingTop: 0 }}>
        <div className="container">
          <h2 className="section__title">Como funciona</h2>
          <ol className="steps">
            <li>
              <h3>Cole a URL</h3>
              <p>Qualquer endereço http ou https, do tamanho que for.</p>
            </li>
            <li>
              <h3>Escolha o acesso</h3>
              <p>Público para todos ou privado, liberado só para quem souber a senha.</p>
            </li>
            <li>
              <h3>Compartilhe e acompanhe</h3>
              <p>Copie o link curto e veja os cliques no seu painel.</p>
            </li>
          </ol>
        </div>
      </section>

      {state.status !== 'authenticated' && (
        <section className="cta">
          <div className="container cta__inner">
            <h2>Pronto para encurtar o primeiro link?</h2>
            <Link to="/cadastro" className="button button--secondary">
              Criar conta grátis
            </Link>
          </div>
        </section>
      )}
    </>
  )
}
