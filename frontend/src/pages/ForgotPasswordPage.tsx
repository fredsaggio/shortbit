import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { confirmPasswordReset, startPasswordReset } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { TextField } from '../components/TextField.tsx'

const minPasswordLength = 8

export function ForgotPasswordPage() {
  const { refresh } = useAuth()
  const navigate = useNavigate()
  const [step, setStep] = useState<'email' | 'code'>('email')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function handleEmail(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await startPasswordReset(email)
      setStep('code')
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleReset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (password !== passwordConfirmation) {
      setError('As senhas não coincidem.')
      return
    }

    setSubmitting(true)
    try {
      await confirmPasswordReset(code, password, passwordConfirmation)
    } catch (submitError) {
      setError(messageOf(submitError))
      setSubmitting(false)
      return
    }

    // The reset revokes every session, including this browser's.
    await refresh()
    navigate('/entrar', { replace: true, state: { email, message: 'Senha alterada. Entre com a nova senha.' } })
  }

  return (
    <section className="auth-page">
      <div className="card auth-card">
        {step === 'email' ? (
          <>
            <header className="auth-card__header">
              <h1>Recuperar senha</h1>
              <p>Informe o email da conta. Se ela existir e tiver senha, enviaremos um código.</p>
            </header>

            {error && <Alert tone="error">{error}</Alert>}

            <form className="form" onSubmit={handleEmail}>
              <TextField
                label="Email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
              <Button type="submit" block loading={submitting}>
                Enviar código
              </Button>
            </form>
          </>
        ) : (
          <>
            <header className="auth-card__header">
              <h1>Nova senha</h1>
              <p>Digite o código de 8 dígitos enviado para {email} e escolha a nova senha.</p>
            </header>

            <Alert tone="info">Se o email não chegar em alguns minutos, confira o spam ou volte e tente de novo.</Alert>
            {error && <Alert tone="error">{error}</Alert>}

            <form className="form" onSubmit={handleReset}>
              <TextField
                label="Código de recuperação"
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{8}"
                maxLength={8}
                placeholder="00000000"
                value={code}
                onChange={(event) => setCode(event.target.value.replace(/\D/g, ''))}
                required
              />
              <TextField
                label="Nova senha"
                type="password"
                autoComplete="new-password"
                minLength={minPasswordLength}
                hint={`Mínimo de ${minPasswordLength} caracteres.`}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
              <TextField
                label="Confirme a nova senha"
                type="password"
                autoComplete="new-password"
                minLength={minPasswordLength}
                value={passwordConfirmation}
                onChange={(event) => setPasswordConfirmation(event.target.value)}
                required
              />
              <Button type="submit" block loading={submitting}>
                Trocar senha
              </Button>
            </form>

            <button type="button" className="text-button" onClick={() => setStep('email')}>
              Usar outro email
            </button>
          </>
        )}

        <p className="auth-card__footer">
          Lembrou a senha? <Link to="/entrar">Entrar</Link>
        </p>
      </div>
    </section>
  )
}
