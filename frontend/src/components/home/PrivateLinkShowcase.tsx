import { ArrowRight, CheckCircle2, Lock, RotateCcw, Sparkles } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { cn } from '@/lib/utils'

const exampleShortCode = 'Qz7Rt2'
const exampleDestination = 'https://drive.exemplo.com/proposta-comercial-2026'

/**
 * Interactive preview of the password prompt served by the backend when a
 * visitor opens a private short link (templates/private_link_password.html).
 */
export function PrivateLinkShowcase() {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [unlocked, setUnlocked] = useState(false)

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!password.trim()) {
      setError('Digite uma senha para testar a prévia.')
      return
    }
    setError('')
    setUnlocked(true)
  }

  function handleSimulate() {
    setPassword('convidados-2026')
    setError('')
    setUnlocked(true)
  }

  function handleReset() {
    setPassword('')
    setError('')
    setUnlocked(false)
  }

  return (
    <div className="mx-auto mt-10 w-full max-w-5xl overflow-hidden rounded-[28px] border border-border bg-card shadow-raised">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-surface px-4 py-3 sm:px-5">
        <div className="flex items-center gap-2" aria-hidden="true">
          <span className="size-3 rounded-full bg-border" />
          <span className="size-3 rounded-full bg-border" />
          <span className="size-3 rounded-full bg-border" />
        </div>

        <div className="flex min-w-0 flex-1 items-center justify-center sm:max-w-md">
          <div className="flex w-full items-center gap-2 truncate rounded-full border border-border bg-card px-3.5 py-1.5 text-xs font-medium text-navy sm:text-sm">
            <Lock
              className={cn('size-3.5 shrink-0', unlocked ? 'text-success' : 'text-brand-700')}
              aria-hidden="true"
            />
            <span className="truncate font-mono text-xs sm:text-sm">
              {unlocked ? exampleDestination : `https://shortbit.com.br/${exampleShortCode}`}
            </span>
          </div>
        </div>

        <button
          type="button"
          onClick={unlocked ? handleReset : handleSimulate}
          className="inline-flex cursor-pointer items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold text-navy transition-colors hover:border-brand/50 hover:text-brand-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          {unlocked ? (
            <>
              <RotateCcw className="size-3.5" aria-hidden="true" />
              Voltar à tela de senha
            </>
          ) : (
            <>
              <Sparkles className="size-3.5 text-brand" aria-hidden="true" />
              Simular desbloqueio
            </>
          )}
        </button>
      </div>

      <div
        className="px-4 pt-5 pb-7 sm:px-8 sm:pt-7 sm:pb-10"
        style={{ background: 'radial-gradient(circle at 8% 16%, #fff0dc 0, transparent 40%), #fff9f1' }}
      >
        <div className="mx-auto flex max-w-[920px] items-center justify-between pb-5 sm:pb-7">
          <span className="inline-flex items-center gap-2.5 font-logo text-xl font-extrabold tracking-tight text-[#17243a] sm:text-2xl">
            <span
              aria-hidden="true"
              className="grid size-8 -rotate-9 place-items-center rounded-[10px] bg-[#f66f1b] text-lg font-black text-white"
            >
              ↗
            </span>
            <span>
              shortbit<span className="text-[#f66f1b]">.</span>
            </span>
          </span>
          <span className="rounded-full border border-[#e8e2db] bg-white/80 px-3 py-1 text-xs font-semibold text-[#56647a]">
            Prévia interativa
          </span>
        </div>

        <div className="mx-auto grid max-w-[920px] overflow-hidden rounded-[22px] border border-[#e8e2db] bg-white shadow-[0_24px_70px_rgba(23,36,58,0.08)] md:grid-cols-2 md:rounded-[26px]">
          <div className="relative flex min-h-[240px] flex-col justify-between overflow-hidden bg-[#17243a] p-7 text-white sm:p-10 md:min-h-[420px]">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute -right-28 -bottom-44 size-80 rounded-full border-[58px] border-[#f66f1b]/95 sm:-right-24 sm:-bottom-40 sm:size-88 sm:border-[68px]"
            />

            <p className="relative z-10 text-[11px] font-extrabold tracking-[0.14em] text-[#ffb47e] uppercase">
              Um espaço só para convidados
            </p>

            <div className="relative z-10 mt-6 md:mt-0">
              <span
                aria-hidden="true"
                className="mb-5 grid size-12 place-items-center rounded-2xl border border-white/20 bg-white/10 text-[#ffb47e] sm:mb-7 sm:size-15 sm:rounded-[20px]"
              >
                <Lock className="size-6 sm:size-7" strokeWidth={1.8} />
              </span>
              <p className="max-w-xs text-[clamp(1.75rem,3vw,2.65rem)] leading-[1.08] font-extrabold tracking-tight">
                Este link tem uma senha.
              </p>
              <p className="mt-3 max-w-xs text-sm leading-relaxed text-[#d7deeb] sm:text-base">
                Quem compartilhou o link com você pode informar a senha de acesso.
              </p>
            </div>
          </div>

          <div className="flex flex-col justify-center p-7 text-left text-[#17243a] sm:p-10">
            {unlocked ? (
              <div className="animate-in fade-in slide-in-from-bottom-2 duration-200">
                <span className="inline-flex items-center gap-1.5 rounded-full bg-success-soft px-3.5 py-1.5 text-xs font-bold text-success">
                  <CheckCircle2 className="size-3.5" aria-hidden="true" />
                  Acesso liberado · Link /{exampleShortCode}
                </span>

                <h3 className="mt-5 text-2xl font-extrabold tracking-tight sm:text-[26px]">
                  Redirecionando ao destino
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-[#56647a]">
                  Na prática, assim que a senha é confirmada, o navegador segue direto para o endereço protegido:
                </p>

                <div className="mt-5 rounded-xl border border-[#e8e2db] bg-[#fff9f1] p-4">
                  <p className="text-[11px] font-bold tracking-wider text-[#9d3b00] uppercase">Destino revelado</p>
                  <p className="mt-1 font-mono text-xs break-all text-[#17243a] sm:text-sm">
                    {exampleDestination}
                  </p>
                </div>

                <button
                  type="button"
                  onClick={handleReset}
                  className="mt-6 inline-flex min-h-12 w-full cursor-pointer items-center justify-center gap-2 rounded-[11px] border border-[#b9c2cf] bg-white px-4 font-bold text-[#17243a] transition-colors hover:bg-[#fff9f1] focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-[#17243a]"
                >
                  <RotateCcw className="size-4" aria-hidden="true" />
                  Ver tela de senha novamente
                </button>
              </div>
            ) : (
              <div>
                <span className="inline-block rounded-full bg-[#fff0e2] px-3.5 py-1.5 text-xs font-bold text-[#9d3b00]">
                  Link /{exampleShortCode}
                </span>

                <h3 className="mt-5 text-2xl font-extrabold tracking-tight sm:text-[26px]">Vamos abrir?</h3>
                <p className="mt-2 text-sm leading-relaxed text-[#56647a] sm:text-[15px]">
                  Digite a senha para continuar até o destino.
                </p>

                <form onSubmit={handleSubmit} noValidate className="mt-6">
                  <span aria-hidden="true" className="mb-2 block text-sm font-bold text-[#17243a]">
                    Senha do link
                  </span>
                  <input
                    type="password"
                    aria-label="Senha da prévia do link privado"
                    autoComplete="off"
                    placeholder="Digite qualquer senha para testar"
                    value={password}
                    onChange={(event) => {
                      setPassword(event.target.value)
                      if (error) setError('')
                    }}
                    aria-invalid={error ? true : undefined}
                    className="min-h-12 w-full rounded-[11px] border border-[#b9c2cf] bg-white px-4 text-sm text-[#17243a] outline-none placeholder:text-[#56647a]/70 focus-visible:border-[#cf4d00] focus-visible:shadow-[0_0_0_3px_rgba(246,111,27,0.22)]"
                  />
                  {error && (
                    <p
                      role="alert"
                      className="mt-3 rounded-[10px] border border-[#efb5a9] bg-[#fff2ef] px-3.5 py-2.5 text-sm text-[#9e2816]"
                    >
                      {error}
                    </p>
                  )}
                  <button
                    type="submit"
                    className="mt-5 inline-flex min-h-12 w-full cursor-pointer items-center justify-center gap-2 rounded-[11px] bg-[#f66f1b] px-4 font-extrabold text-[#1b2534] transition-[background-color,transform] duration-150 hover:bg-[#ff8a3d] active:translate-y-px focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-[#17243a]"
                  >
                    Acessar link
                    <ArrowRight className="size-4" aria-hidden="true" />
                  </button>
                </form>

                <p className="mt-5 text-center text-xs leading-relaxed text-[#56647a]">
                  A senha libera apenas este link neste navegador.
                </p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
