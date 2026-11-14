import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { confirmRegistration, resendRegistrationCode, startRegistration } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { GoogleButton } from '../components/GoogleButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkCode, checkConfirmation, checkEmail, checkNewPassword, minPasswordLength } from '../validation.ts'

const resendCooldownSeconds = 30

export function RegisterPage() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const introMessage = (location.state as { message?: string } | null)?.message

  const [step, setStep] = useState<'details' | 'code'>('details')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [code, setCode] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [resendIn, setResendIn] = useState(0)
  const fields = useFieldErrors<'email' | 'password' | 'passwordConfirmation' | 'code'>()

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
      email: checkEmail(email),
      password: checkNewPassword(password),
      passwordConfirmation: checkConfirmation(password, passwordConfirmation),
    })
    if (!valid) {
      return
    }

    setSubmitting(true)
    try {
      await startRegistration(email.trim(), password)
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

  return (
    <section className="auth-page">
      <div className="card auth-card">
        {step === 'details' ? (
          <>
            <header className="auth-card__header">
              <h1>Criar conta</h1>
              <p>Grátis. Leva menos de um minuto.</p>
            </header>

            {introMessage && <Alert tone="info">{introMessage}</Alert>}
            {error && <Alert tone="error">{error}</Alert>}

            <GoogleButton label="Criar conta com Google" />
            <div className="divider">ou com email</div>

            <form className="form" onSubmit={handleDetails} noValidate>
              <TextField
                label="Email"
                name="email"
                type="email"
                autoComplete="email"
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
              <Button type="submit" block loading={submitting}>
                Continuar
              </Button>
            </form>

            <p className="auth-card__footer">
              Já tem conta? <Link to="/entrar">Entrar</Link>
            </p>
          </>
        ) : (
          <>
            <header className="auth-card__header">
              <h1>Confirme seu email</h1>
              <p>Digite o código que chegou na sua caixa de entrada.</p>
            </header>

            {notice && <Alert tone="success">{notice}</Alert>}
            {error && <Alert tone="error">{error}</Alert>}

            <form className="form" onSubmit={handleCode} noValidate>
              <TextField
                label="Código de confirmação"
                name="code"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                placeholder="000000"
                value={code}
                error={fields.errors.code}
                onChange={(event) => {
                  setCode(event.target.value.replace(/\D/g, ''))
                  fields.clear('code')
                }}
                required
              />
              <Button type="submit" block loading={submitting}>
                Confirmar e entrar
              </Button>
            </form>

            <div className="row-between">
              <button type="button" className="text-button" onClick={handleResend} disabled={resendIn > 0}>
                {resendIn > 0 ? `Reenviar código em ${resendIn}s` : 'Reenviar código'}
              </button>
              <button type="button" className="text-button" onClick={restart}>
                Usar outro email
              </button>
            </div>
          </>
        )}
      </div>
    </section>
  )
}
