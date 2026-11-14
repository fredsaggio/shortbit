import { MailCheck } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { confirmRegistration, resendRegistrationCode, startRegistration } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { AuthCard, OrDivider, textLinkClass } from '../components/AuthCard.tsx'
import { Button } from '../components/Button.tsx'
import { GoogleButton } from '../components/GoogleButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkCode, checkConfirmation, checkEmail, checkFullName, checkNewPassword, maxFullNameLength, minPasswordLength } from '../validation.ts'

const resendCooldownSeconds = 30

function StepIndicator({ step }: { step: 1 | 2 }) {
  return (
    <div className="flex items-center gap-2" aria-label={`Etapa ${step} de 2`}>
      {[1, 2].map((value) => (
        <span
          key={value}
          className={`h-1.5 flex-1 rounded-full transition-colors duration-300 ${value <= step ? 'bg-brand' : 'bg-surface-200'}`}
        />
      ))}
    </div>
  )
}

export function RegisterPage() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const introMessage = (location.state as { message?: string } | null)?.message

  const [step, setStep] = useState<'details' | 'code'>('details')
  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [code, setCode] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [resendIn, setResendIn] = useState(0)
  const fields = useFieldErrors<'fullName' | 'email' | 'password' | 'passwordConfirmation' | 'code'>()

  useEffect(() => {
    if (resendIn <= 0) {
      return
    }
    const timer = window.setTimeout(() => setResendIn((seconds) => seconds - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [resendIn])

  async function handleDetails(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const valid = fields.check(event.currentTarget, {
      fullName: checkFullName(fullName),
      email: checkEmail(email),
      password: checkNewPassword(password),
      passwordConfirmation: checkConfirmation(password, passwordConfirmation),
    })
    if (!valid) {
      return
    }

    setSubmitting(true)
    try {
      await startRegistration(email.trim(), password, fullName.trim())
      setStep('code')
      setNotice(`Enviamos um código de 6 dígitos para ${email.trim()}.`)
      setResendIn(resendCooldownSeconds)
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleCode(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (!fields.check(event.currentTarget, { code: checkCode(code, 6) })) {
      return
    }

    setSubmitting(true)
    try {
      await confirmRegistration(code)
    } catch (submitError) {
      setError(messageOf(submitError))
      setSubmitting(false)
      return
    }

    // The account exists now; signing in with the password typed a moment
    // ago saves the visitor a second form.
    try {
      await signIn(email.trim(), password, false)
      navigate('/painel', { replace: true })
    } catch {
      navigate('/entrar', { replace: true, state: { email: email.trim(), message: 'Conta criada! Entre com seu email e senha.' } })
    }
  }

  async function handleResend() {
    setError('')
    setNotice('')
    try {
      await resendRegistrationCode()
      setNotice(`Enviamos um novo código para ${email.trim()}.`)
      setResendIn(resendCooldownSeconds)
    } catch (resendError) {
      setError(messageOf(resendError))
    }
  }

  function restart() {
    setStep('details')
    setCode('')
    setError('')
    setNotice('')
  }

  if (step === 'code') {
    return (
      <AuthCard
        title="Confirme seu email"
        description={
          <>
            Digite o código que enviamos para <strong className="font-semibold text-navy">{email.trim()}</strong>.
          </>
        }
      >
        <StepIndicator step={2} />
        <div className="flex justify-center py-1" aria-hidden="true">
          <span className="inline-flex size-14 items-center justify-center rounded-2xl bg-brand-100">
            <MailCheck className="size-7 text-brand" />
          </span>
        </div>
        {notice && <Alert tone="success">{notice}</Alert>}
        {error && <Alert tone="error">{error}</Alert>}

        <form className="grid gap-5" onSubmit={handleCode} noValidate>
          <TextField
            label="Código de confirmação"
            name="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            placeholder="000000"
            className="[&_input]:text-center [&_input]:text-2xl [&_input]:font-bold [&_input]:tracking-[0.4em]"
            value={code}
            error={fields.errors.code}
            onChange={(event) => {
              setCode(event.target.value.replace(/\D/g, ''))
              fields.clear('code')
            }}
            required
          />
          <Button type="submit" size="lg" block loading={submitting}>
            Confirmar e entrar
          </Button>
        </form>

        <div className="flex flex-wrap items-center justify-between gap-3 text-[15px]">
          <button type="button" className={`${textLinkClass} disabled:cursor-not-allowed disabled:text-muted-foreground disabled:no-underline`} onClick={handleResend} disabled={resendIn > 0}>
            {resendIn > 0 ? `Reenviar código em ${resendIn}s` : 'Reenviar código'}
          </button>
          <button type="button" className={textLinkClass} onClick={restart}>
            Usar outro email
          </button>
        </div>
      </AuthCard>
    )
  }

  return (
    <AuthCard
      title="Crie sua conta grátis"
      description="Leva menos de um minuto. Sem cartão de crédito."
      footer={
        <>
          Já tem conta?{' '}
          <Link to="/entrar" className={textLinkClass}>
            Entrar
          </Link>
        </>
      }
    >
      <StepIndicator step={1} />
      {introMessage && <Alert tone="info">{introMessage}</Alert>}
      {error && <Alert tone="error">{error}</Alert>}

      <GoogleButton label="Criar conta com Google" />
      <OrDivider label="ou com email" />

      <form className="grid gap-5" onSubmit={handleDetails} noValidate>
        <TextField
          label="Nome completo"
          name="fullName"
          autoComplete="name"
          placeholder="Como você quer ser chamado"
          maxLength={maxFullNameLength}
          value={fullName}
          error={fields.errors.fullName}
          onChange={(event) => {
            setFullName(event.target.value)
            fields.clear('fullName')
          }}
          required
        />
        <TextField
          label="Email"
          name="email"
          type="email"
          autoComplete="email"
          placeholder="voce@exemplo.com"
          value={email}
          error={fields.errors.email}
          onChange={(event) => {
            setEmail(event.target.value)
            fields.clear('email')
          }}
          required
        />
        <TextField
          label="Senha"
          name="password"
          type="password"
          autoComplete="new-password"
          hint={`Mínimo de ${minPasswordLength} caracteres.`}
          value={password}
          error={fields.errors.password}
          onChange={(event) => {
            setPassword(event.target.value)
            fields.clear('password')
          }}
          required
        />
        <TextField
          label="Confirme a senha"
          name="passwordConfirmation"
          type="password"
          autoComplete="new-password"
          value={passwordConfirmation}
          error={fields.errors.passwordConfirmation}
          onChange={(event) => {
            setPasswordConfirmation(event.target.value)
            fields.clear('passwordConfirmation')
          }}
          required
        />
        <Button type="submit" size="lg" block loading={submitting}>
          Continuar
        </Button>
      </form>
    </AuthCard>
  )
}
