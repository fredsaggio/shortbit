import { useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router'
import { messageOf } from '../api/client.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { GoogleButton } from '../components/GoogleButton.tsx'
import { TextField } from '../components/TextField.tsx'
import { returnPath } from '../navigation.ts'

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
  const [error, setError] = useState(
    searchParams.get('erro') === 'google' ? 'Não foi possível entrar com o Google. Tente de novo.' : '',
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await signIn(email, password, rememberMe)
      navigate(returnPath(location.state), { replace: true })
    } catch (submitError) {
      setError(messageOf(submitError))
      setSubmitting(false)
    }
  }

  return (
    <section className="auth-page">
      <div className="card auth-card">
        <header className="auth-card__header">
          <h1>Entrar</h1>
          <p>Acesse seus links e veja os cliques.</p>
        </header>

        {locationState?.message && <Alert tone="success">{locationState.message}</Alert>}
        {error && <Alert tone="error">{error}</Alert>}

        <GoogleButton />
        <div className="divider">ou com email</div>

        <form className="form" onSubmit={handleSubmit}>
          <TextField
            label="Email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
          <TextField
            label="Senha"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
          <div className="row-between">
            <label className="checkbox">
              <input type="checkbox" checked={rememberMe} onChange={(event) => setRememberMe(event.target.checked)} />
              Manter conectado por 30 dias
            </label>
            <Link to="/recuperar-senha">Esqueci a senha</Link>
          </div>
          <Button type="submit" block loading={submitting}>
            Entrar
          </Button>
        </form>

        <p className="auth-card__footer">
          Ainda não tem conta? <Link to="/cadastro">Criar conta grátis</Link>
        </p>
      </div>
    </section>
  )
}
