import { useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { AuthCard, OrDivider, textLinkClass } from '../components/AuthCard.tsx'
import { Button } from '../components/Button.tsx'
import { GoogleButton } from '../components/GoogleButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { returnPath } from '../navigation.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkEmail, checkRequired } from '../validation.ts'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

type LoginLocationState = { message?: string; email?: string } | null

export function LoginPage() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const locationState = location.state as LoginLocationState

  const [email, setEmail] = useState(locationState?.email ?? '')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const fields = useFieldErrors<'email' | 'password'>()
  const [error, setError] = useState(
    searchParams.get('erro') === 'google' ? 'Não foi possível entrar com o Google. Tente de novo.' : '',
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const valid = fields.check(event.currentTarget, {
      email: checkEmail(email),
      password: checkRequired(password, 'Informe sua senha.'),
    })
    if (!valid) {
      return
    }

    setSubmitting(true)
    try {
      await signIn(email.trim(), password, rememberMe)
      navigate(returnPath(location.state), { replace: true })
    } catch (submitError) {
      setError(messageOf(submitError))
      setSubmitting(false)
    }
  }

  return (
    <AuthCard
      title="Entrar"
      description="Que bom te ver de novo. Acesse seus links e cliques."
      footer={
        <>
          Ainda não tem conta?{' '}
          <Link to="/cadastro" className={textLinkClass}>
            Criar conta grátis
          </Link>
        </>
      }
    >
      {locationState?.message && <Alert tone="success">{locationState.message}</Alert>}
      {error && <Alert tone="error">{error}</Alert>}

      <GoogleButton />
      <OrDivider label="ou com email" />

      <form className="grid gap-5" onSubmit={handleSubmit} noValidate>
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
          autoComplete="current-password"
          value={password}
          error={fields.errors.password}
          onChange={(event) => {
            setPassword(event.target.value)
            fields.clear('password')
          }}
          required
        />
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <Checkbox
              id="remember-me"
              checked={rememberMe}
              onCheckedChange={(checked) => setRememberMe(checked === true)}
              className="size-5 rounded-[5px] border-input data-[state=checked]:border-action data-[state=checked]:bg-action"
            />
            <Label htmlFor="remember-me" className="cursor-pointer text-[15px] font-medium text-navy">
              Manter conectado por 30 dias
            </Label>
          </div>
          <Link to="/recuperar-senha" className={textLinkClass}>
            Esqueci a senha
          </Link>
        </div>
        <Button type="submit" size="lg" block loading={submitting}>
          Entrar
        </Button>
      </form>
    </AuthCard>
  )
}
