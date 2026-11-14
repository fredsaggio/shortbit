import { useId, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { confirmPasswordReset, startPasswordReset } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { AuthCard, textLinkClass } from '../components/AuthCard.tsx'
import { Button } from '../components/Button.tsx'
import { PasswordChecklist } from '../components/PasswordChecklist.tsx'
import { TextField } from '../components/TextField.tsx'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkCode, checkConfirmation, checkEmail, checkNewPassword, passwordRules } from '../validation.ts'

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
  const fields = useFieldErrors<'email' | 'code' | 'password' | 'passwordConfirmation'>()
  const checklistId = useId()

  const footer = (
    <>
      Lembrou a senha?{' '}
      <Link to="/entrar" className={textLinkClass}>
        Entrar
      </Link>
    </>
  )

  async function handleEmail(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (!fields.check(event.currentTarget, { email: checkEmail(email) })) {
      return
    }

    setSubmitting(true)
    try {
      await startPasswordReset(email.trim())
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
    const valid = fields.check(event.currentTarget, {
      code: checkCode(code, 8),
      password: checkNewPassword(password),
      passwordConfirmation: checkConfirmation(password, passwordConfirmation),
    })
    if (!valid) {
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
    navigate('/entrar', { replace: true, state: { email: email.trim(), message: 'Senha alterada. Entre com a nova senha.' } })
  }

  if (step === 'email') {
    return (
      <AuthCard title="Recuperar senha" description="Informe o email da conta. Se ela existir e tiver senha, enviaremos um código." footer={footer}>
        {error && <Alert tone="error">{error}</Alert>}
        <form className="grid gap-5" onSubmit={handleEmail} noValidate>
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
          <Button type="submit" size="lg" block loading={submitting}>
            Enviar código
          </Button>
        </form>
      </AuthCard>
    )
  }

  return (
    <AuthCard
      title="Crie uma nova senha"
      description={
        <>
          Digite o código de 8 dígitos enviado para <strong className="font-semibold text-navy">{email.trim()}</strong>.
        </>
      }
      footer={footer}
    >
      <Alert tone="info">Se o email não chegar em alguns minutos, confira o spam ou volte e tente de novo.</Alert>
      {error && <Alert tone="error">{error}</Alert>}

      <form className="grid gap-5" onSubmit={handleReset} noValidate>
        <TextField
          label="Código de recuperação"
          name="code"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={8}
          placeholder="00000000"
          className="[&_input]:text-center [&_input]:text-xl [&_input]:font-bold [&_input]:tracking-[0.3em]"
          value={code}
          error={fields.errors.code}
          onChange={(event) => {
            setCode(event.target.value.replace(/\D/g, ''))
            fields.clear('code')
          }}
          required
        />
        <TextField
          label="Nova senha"
          name="password"
          type="password"
          autoComplete="new-password"
          describedBy={checklistId}
          value={password}
          error={fields.errors.password}
          onChange={(event) => {
            setPassword(event.target.value)
            fields.clear('password')
          }}
          required
        />
        <TextField
          label="Confirme a nova senha"
          name="passwordConfirmation"
          type="password"
          autoComplete="new-password"
          describedBy={checklistId}
          value={passwordConfirmation}
          error={fields.errors.passwordConfirmation}
          onChange={(event) => {
            setPasswordConfirmation(event.target.value)
            fields.clear('passwordConfirmation')
          }}
          required
        />
        <PasswordChecklist id={checklistId} rules={passwordRules({ password, confirmation: passwordConfirmation })} />
        <Button type="submit" size="lg" block loading={submitting}>
          Trocar senha
        </Button>
      </form>

      <button type="button" className={`${textLinkClass} justify-self-start text-[15px]`} onClick={() => setStep('email')}>
        Usar outro email
      </button>
    </AuthCard>
  )
}
