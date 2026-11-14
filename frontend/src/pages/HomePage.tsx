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
import { ClickBars } from '../components/home/ClickBars.tsx'
import { FloatingLinks } from '../components/home/FloatingLinks.tsx'
import { LinkShrinkDemo } from '../components/home/LinkShrinkDemo.tsx'
import { Reveal } from '../components/Reveal.tsx'
import { TextField } from '../components/TextField.tsx'
import { useInView } from '../motion.ts'
import { savePendingLink } from '../pendingLink.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkURL } from '../validation.ts'
import { cn } from '@/lib/utils'

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
      navigate('/cadastro', { state: { message: 'Crie sua conta grátis ou entre: o seu link curto fica pronto assim que você entrar.' } })
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
    <div className="mx-auto mt-10 w-full max-w-4xl rounded-[28px] border border-white/10 bg-white/5 p-1.5 shadow-raised motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-6 motion-safe:duration-700 motion-safe:[animation-delay:450ms] motion-safe:[animation-fill-mode:both] sm:mt-12 sm:p-2">
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

const headlineWords = ['Links', 'curtos', 'que', 'mostram', 'quem']

// Each word rises in turn, then an orange stroke is drawn under "clicou".
function HeroTitle() {
  return (
    <h1 className="mx-auto max-w-4xl text-[clamp(2.25rem,6vw,4.25rem)] leading-[1.05] font-extrabold tracking-tight">
      {headlineWords.map((word, index) => (
        <span key={word}>
          <span
            className="inline-block motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-4 motion-safe:duration-700 motion-safe:[animation-fill-mode:both]"
            style={{ animationDelay: `${index * 70}ms` }}
          >
            {word}
          </span>{' '}
        </span>
      ))}
      <span
        className="relative inline-block text-brand motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-4 motion-safe:duration-700 motion-safe:[animation-fill-mode:both]"
        style={{ animationDelay: `${headlineWords.length * 70}ms` }}
      >
        clicou
        <svg aria-hidden="true" viewBox="0 0 200 20" preserveAspectRatio="none" className="absolute -bottom-1 left-0 h-3 w-full overflow-visible sm:-bottom-2 sm:h-4">
          <path
            d="M3 15 C 55 3, 130 3, 197 11"
            fill="none"
            stroke="currentColor"
            strokeWidth="5"
            strokeLinecap="round"
            pathLength={1}
            className="[stroke-dasharray:1] [stroke-dashoffset:1] motion-safe:animate-draw motion-reduce:[stroke-dashoffset:0]"
            style={{ animationDelay: '750ms' }}
          />
        </svg>
      </span>
    </h1>
  )
}

// Three steps joined by an orange line that draws itself across, each number
// popping in as the line reaches it.
function HowItWorks() {
  const [ref, inView] = useInView<HTMLOListElement>({ threshold: 0.35 })

  return (
    <ol ref={ref} className="relative mt-14 grid gap-10 md:grid-cols-3 md:gap-8">
      <span
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute top-5 right-[16.66%] left-[16.66%] hidden h-0.5 origin-left bg-brand/40 md:block',
          'motion-safe:transition-transform motion-safe:duration-1000 motion-safe:ease-[cubic-bezier(0.65,0,0.35,1)]',
          inView ? 'scale-x-100' : 'motion-safe:scale-x-0',
        )}
      />
      {steps.map((step, index) => (
        <li key={step.title} className="relative text-center">
          <span
            style={{ transitionDelay: inView ? `${index * 420}ms` : undefined }}
            className={cn(
              'relative z-10 mx-auto inline-flex size-10 items-center justify-center rounded-full bg-brand text-lg font-extrabold text-white shadow-sm ring-8 ring-card',
              'motion-safe:transition-[transform,opacity] motion-safe:duration-500 motion-safe:ease-[cubic-bezier(0.34,1.56,0.64,1)]',
              inView ? 'scale-100 opacity-100' : 'motion-safe:scale-50 motion-safe:opacity-0',
            )}
          >
            {index + 1}
          </span>
          <div
            style={{ transitionDelay: inView ? `${index * 420 + 150}ms` : undefined }}
            className={cn(
              'motion-safe:transition-[transform,opacity] motion-safe:duration-700 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)]',
              inView ? 'translate-y-0 opacity-100' : 'motion-safe:translate-y-4 motion-safe:opacity-0',
            )}
          >
            <h3 className="mt-5 text-xl font-bold">{step.title}</h3>
            <p className="mx-auto mt-2 max-w-xs text-muted-foreground">{step.text}</p>
          </div>
        </li>
      ))}
    </ol>
  )
}

function VisibilityPills() {
  const [ref, inView] = useInView<HTMLDivElement>({ threshold: 0.6 })
  const pills = [
    { label: 'shortbit/Ab3dX9 · público', className: 'bg-navy-100 text-navy' },
    { label: 'shortbit/Qz7Rt2 · com senha', className: 'bg-brand-100 text-brand-700' },
  ]

  return (
    <div ref={ref} className="mt-6 flex flex-wrap gap-2" aria-hidden="true">
      {pills.map((pill, index) => (
        <span
          key={pill.label}
          style={{ transitionDelay: inView ? `${200 + index * 150}ms` : undefined }}
          className={cn(
            'rounded-full px-3 py-1 text-sm font-semibold',
            'motion-safe:transition-[transform,opacity] motion-safe:duration-500 motion-safe:ease-[cubic-bezier(0.22,1,0.36,1)]',
            inView ? 'translate-x-0 opacity-100' : 'motion-safe:-translate-x-4 motion-safe:opacity-0',
            pill.className,
          )}
        >
          {pill.label}
        </span>
      ))}
    </div>
  )
}

const cardClass = 'group h-full rounded-3xl border p-7 shadow-card transition-[transform,box-shadow] duration-300 hover:-translate-y-1 hover:shadow-raised'

export function HomePage() {
  const { state } = useAuth()

  return (
    <>
      <section className="relative overflow-hidden bg-navy text-white">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-40 left-1/2 h-[480px] w-[900px] -translate-x-1/2 rounded-full bg-action/25 blur-3xl"
        />
        <FloatingLinks />
        <div className="relative mx-auto max-w-7xl px-4 pt-14 pb-16 text-center sm:px-6 sm:pt-20 sm:pb-20">
          <HeroTitle />
          <p className="mx-auto mt-6 max-w-2xl text-lg text-navy-300 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-700 motion-safe:[animation-delay:300ms] motion-safe:[animation-fill-mode:both] sm:text-xl">
            Transforme URLs enormes em links fáceis de compartilhar, proteja os que precisam de senha e acompanhe cada clique.
          </p>

          <ShortenCard />
          <LinkShrinkDemo />
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-4 py-20 sm:px-6">
        <Reveal className="mx-auto max-w-3xl text-center">
          <p className="text-sm font-bold tracking-widest text-brand-700 uppercase">Por que shortbit</p>
          <h2 className="mt-3 text-[clamp(1.875rem,4vw,3rem)] leading-tight font-extrabold">Tudo o que um link curto precisa</h2>
          <p className="mt-4 text-lg text-muted-foreground">Crie a conta e comece a encurtar em menos de um minuto.</p>
        </Reveal>

        <div className="mt-12 grid gap-4 md:grid-cols-3">
          <Reveal className="md:col-span-2">
            <article className={cn(cardClass, 'bg-card')}>
              <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand-700 uppercase">
                <Lock className="size-5 text-brand transition-transform duration-300 group-hover:-rotate-12" aria-hidden="true" /> Privacidade
              </span>
              <h3 className="mt-3 text-2xl font-bold">Público ou com senha</h3>
              <p className="mt-2 max-w-lg text-muted-foreground">
                Compartilhe um link aberto para todo mundo ou proteja o destino com uma senha. Quem não souber a senha não vê para
                onde o link leva.
              </p>
              <VisibilityPills />
            </article>
          </Reveal>

          <Reveal delay={120}>
            <article className={cn(cardClass, 'bg-card')}>
              <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand-700 uppercase">
                <BarChart3 className="size-5 text-brand" aria-hidden="true" /> Cliques
              </span>
              <h3 className="mt-3 text-2xl font-bold">Cada acesso contado</h3>
              <p className="mt-2 text-muted-foreground">Veja no painel quantas vezes cada link foi aberto.</p>
              <ClickBars />
            </article>
          </Reveal>

          <Reveal delay={60} className="md:col-span-3">
            <article className={cn(cardClass, 'bg-navy text-white md:flex md:items-center md:justify-between md:gap-10')}>
              <div>
                <span className="inline-flex items-center gap-2 text-sm font-bold tracking-wide text-brand uppercase">
                  <Zap className="size-5 transition-transform duration-300 group-hover:scale-125 group-hover:-rotate-6" aria-hidden="true" /> Desempenho
                </span>
                <h3 className="mt-3 text-2xl font-bold">Rápido mesmo quando o link viraliza</h3>
                <p className="mt-2 max-w-2xl text-navy-300">
                  Os links mais acessados saem direto da memória, e a contagem roda em segundo plano para o redirecionamento não
                  esperar.
                </p>
              </div>
              <Link
                to={state.status === 'authenticated' ? '/painel' : '/cadastro'}
                className={cn('group/cta mt-6 inline-flex shrink-0 items-center gap-2 rounded-md font-semibold transition-colors md:mt-0', buttonSizes.md, buttonTones.onDark)}
              >
                {state.status === 'authenticated' ? 'Ir para o painel' : 'Começar grátis'}
                <ArrowRight className="size-4 transition-transform duration-200 group-hover/cta:translate-x-1" aria-hidden="true" />
              </Link>
            </article>
          </Reveal>
        </div>
      </section>

      <section className="border-t bg-card">
        <div className="mx-auto max-w-7xl px-4 py-20 sm:px-6">
          <Reveal>
            <h2 className="text-center text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">Como funciona</h2>
          </Reveal>
          <HowItWorks />
        </div>
      </section>

      {state.status !== 'authenticated' && (
        <section className="bg-surface">
          <Reveal className="mx-auto flex max-w-7xl flex-col items-center gap-6 px-4 py-16 text-center sm:px-6">
            <h2 className="text-[clamp(1.75rem,4vw,2.5rem)] font-extrabold">Pronto para encurtar o primeiro link?</h2>
            <Link
              to="/cadastro"
              className={cn('group/cta inline-flex items-center gap-2 rounded-md font-semibold transition-colors', buttonSizes.lg, buttonTones.primary)}
            >
              Criar conta grátis
              <ArrowRight className="size-4 transition-transform duration-200 group-hover/cta:translate-x-1" aria-hidden="true" />
            </Link>
          </Reveal>
        </section>
      )}
    </>
  )
}
