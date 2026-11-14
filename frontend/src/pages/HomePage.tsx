import { ArrowRight, BarChart3, CircleCheck, Lock, Zap } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { messageOf } from '../api/client.ts'
import { createLink } from '../api/links.ts'
import type { CreatedLink } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { buttonSizes, buttonTones } from '../components/buttonStyles.ts'
import { CopyButton } from '../components/CopyButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { savePendingLink } from '../pendingLink.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkURL } from '../validation.ts'
import { cn } from '@/lib/utils'

const freePlan = ['Links públicos e com senha', 'Cliques ilimitados', 'Painel com a contagem de cada link']

const steps = [
  { title: 'Cole a URL', text: 'Qualquer endereço http ou https, do tamanho que for.' },
  { title: 'Escolha o acesso', text: 'Aberto para todo mundo ou liberado só com senha.' },
  { title: 'Compartilhe e acompanhe', text: 'Copie o link curto e veja os cliques no painel.' },
]

function ShortenCard() {
  const { state } = useAuth()
  const navigate = useNavigate()
  const [url, setUrl] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<CreatedLink | null>(null)
  const fields = useFieldErrors<'url'>()

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setCreated(null)
    if (!fields.check(event.currentTarget, { url: checkURL(url) })) {
      return
    }

    if (state.status !== 'authenticated') {
      savePendingLink(url.trim())
      navigate('/cadastro', { state: { message: 'Crie sua conta grátis para encurtar o seu link. Ele já vai estar no painel.' } })
      return
    }

    setSubmitting(true)
    try {
      setCreated(await createLink({ url: url.trim(), visibility: 'public' }))
      setUrl('')
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto mt-10 w-full max-w-4xl rounded-[28px] border border-white/10 bg-white/5 p-1.5 shadow-raised sm:mt-12 sm:p-2">
      <div className="rounded-[22px] bg-card p-5 text-left sm:p-8">
        <h2 className="text-xl font-bold text-navy sm:text-2xl">Encurte um link longo de graça</h2>
        <form className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-start" onSubmit={handleSubmit} noValidate>
          <TextField
            label="URL para encurtar"
            hideLabel
            name="url"
            type="url"
            inputMode="url"
            placeholder="Cole aqui o seu link longo"
            className="flex-1"
            value={url}
            error={fields.errors.url}
            onChange={(event) => {
              setUrl(event.target.value)
              fields.clear('url')
            }}
            required
          />
          <Button type="submit" size="lg" loading={submitting} className="sm:min-w-44">
            Encurtar
          </Button>
        </form>
        {error && <Alert tone="error" className="mt-4">{error}</Alert>}
        {created && (
          <div className="mt-4 flex flex-wrap items-center gap-3 rounded-xl border border-success/25 bg-success-soft p-4 animate-in fade-in zoom-in-95 duration-200">
            <CircleCheck className="size-5 text-success" aria-hidden="true" />
            <a href={created.short_url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate font-bold text-action hover:underline">
              {created.short_url}
            </a>
            <CopyButton text={created.short_url} />
          </div>
        )}
        <p className="mt-5 text-sm text-muted-foreground">
          {state.status === 'authenticated' ? 'O link vai direto para o seu painel.' : 'Você cria a conta em menos de um minuto, sem cartão.'}
        </p>
      </div>
    </div>
  )
}

export function HomePage() {
  const { state } = useAuth()

  return (
    <>
      <section className="relative overflow-hidden bg-navy text-white">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-40 left-1/2 h-[480px] w-[900px] -translate-x-1/2 rounded-full bg-action/25 blur-3xl"
        />
        <div className="relative mx-auto max-w-7xl px-4 pt-14 pb-16 text-center sm:px-6 sm:pt-20 sm:pb-20">
          <h1 className="mx-auto max-w-4xl text-[clamp(2.25rem,6vw,4.25rem)] leading-[1.05] font-extrabold tracking-tight">
            Links curtos que mostram quem <span className="text-brand">clicou</span>
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-lg text-navy-300 sm:text-xl">
            Transforme URLs enormes em links fáceis de compartilhar, proteja os que precisam de senha e acompanhe cada clique.
          </p>

          <ShortenCard />

          <ul className="mx-auto mt-10 flex max-w-4xl flex-col items-center justify-center gap-x-8 gap-y-3 text-[15px] font-medium text-navy-300 sm:flex-row sm:flex-wrap">
            <li className="font-bold text-white">Seu plano grátis inclui:</li>
            {freePlan.map((item) => (
              <li key={item} className="inline-flex items-center gap-2">
                <CircleCheck className="size-5 text-brand" aria-hidden="true" />
                {item}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-4 py-20 sm:px-6">
        <div className="mx-auto max-w-3xl text-center">
          <p className="text-sm font-bold tracking-widest text-brand-700 uppercase">Por que shortbit</p>
          <h2 className="mt-3 text-[clamp(1.875rem,4vw,3rem)] leading-tight font-extrabold">Tudo o que um link curto precisa</h2>
          <p className="mt-4 text-lg text-muted-foreground">Sem planos complicados. Crie a conta e comece a encurtar.</p>
        </div>

        <div className="mt-12 grid gap-4 md:grid-cols-3">
          <article className="group rounded-3xl border bg-card p-7 shadow-card transition-[transform,box-shadow] duration-200 hover:-translate-y-1 hover:shadow-raised md:col-span-2">
            <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand-700 uppercase">
              <Lock className="size-5 text-brand" aria-hidden="true" /> Privacidade
            </span>
            <h3 className="mt-3 text-2xl font-bold">Público ou com senha</h3>
            <p className="mt-2 max-w-lg text-muted-foreground">
              Compartilhe um link aberto para todo mundo ou proteja o destino com uma senha. Quem não souber a senha não vê para
              onde o link leva.
            </p>
            <div className="mt-6 flex flex-wrap gap-2" aria-hidden="true">
              <span className="rounded-full bg-navy-100 px-3 py-1 text-sm font-semibold text-navy">shortbit/Ab3dX9 · público</span>
              <span className="rounded-full bg-brand-100 px-3 py-1 text-sm font-semibold text-brand-700">shortbit/Qz7Rt2 · com senha</span>
            </div>
          </article>

          <article className="group rounded-3xl border bg-card p-7 shadow-card transition-[transform,box-shadow] duration-200 hover:-translate-y-1 hover:shadow-raised">
            <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand-700 uppercase">
              <BarChart3 className="size-5 text-brand" aria-hidden="true" /> Cliques
            </span>
            <h3 className="mt-3 text-2xl font-bold">Cada acesso contado</h3>
            <p className="mt-2 text-muted-foreground">Veja no painel quantas vezes cada link foi aberto.</p>
          </article>

          <article className="group rounded-3xl border bg-navy p-7 text-white shadow-card transition-[transform,box-shadow] duration-200 hover:-translate-y-1 hover:shadow-raised md:col-span-3 md:flex md:items-center md:justify-between md:gap-10">
            <div>
              <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand uppercase">
                <Zap className="size-5" aria-hidden="true" /> Desempenho
              </span>
              <h3 className="mt-3 text-2xl font-bold">Rápido mesmo quando o link viraliza</h3>
              <p className="mt-2 max-w-2xl text-navy-300">
                Os links mais acessados saem direto da memória, e a contagem roda em segundo plano para o redirecionamento não
                esperar.
              </p>
            </div>
            <Link
              to={state.status === 'authenticated' ? '/painel' : '/cadastro'}
              className={cn('mt-6 inline-flex shrink-0 items-center gap-2 rounded-md font-semibold transition-colors md:mt-0', buttonSizes.md, buttonTones.onDark)}
            >
              {state.status === 'authenticated' ? 'Ir para o painel' : 'Começar grátis'}
              <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
          </article>
        </div>
      </section>

      <section className="border-t bg-card">
        <div className="mx-auto max-w-7xl px-4 py-20 sm:px-6">
          <h2 className="text-center text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">Como funciona</h2>
          <ol className="mt-12 grid gap-6 md:grid-cols-3">
            {steps.map((step, index) => (
              <li key={step.title} className="relative rounded-3xl border bg-surface p-7">
                <span className="inline-flex size-10 items-center justify-center rounded-full bg-brand text-lg font-extrabold text-white">
                  {index + 1}
                </span>
                <h3 className="mt-4 text-xl font-bold">{step.title}</h3>
                <p className="mt-2 text-muted-foreground">{step.text}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {state.status !== 'authenticated' && (
        <section className="bg-surface">
          <div className="mx-auto flex max-w-7xl flex-col items-center gap-6 px-4 py-16 text-center sm:px-6">
            <h2 className="text-[clamp(1.75rem,4vw,2.5rem)] font-extrabold">Pronto para encurtar o primeiro link?</h2>
            <Link to="/cadastro" className={cn('inline-flex items-center gap-2 rounded-md font-semibold transition-colors', buttonSizes.lg, buttonTones.primary)}>
              Criar conta grátis
              <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
          </div>
        </section>
      )}
    </>
  )
}
