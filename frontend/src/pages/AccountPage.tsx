import { useState, type FormEvent } from 'react'
import { changePassword, updateFullName } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import type { Account } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { PageLoader } from '../components/PageLoader.tsx'
import { TextField } from '../components/TextField.tsx'
import { formatLongDate } from '../format.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkConfirmation, checkFullName, checkNewPassword, checkRequired, maxFullNameLength, minPasswordLength } from '../validation.ts'

function SignInMethods({ account }: { account: Account }) {
  return (
    <span className="methods">
      {account.has_password && <span className="badge badge--public">Email e senha</span>}
      {account.has_google && <span className="badge badge--google">Google</span>}
    </span>
  )
}

function ProfileCard({ account }: { account: Account }) {
  const { replaceAccount } = useAuth()
  const [fullName, setFullName] = useState(account.full_name ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const fields = useFieldErrors<'fullName'>()

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSaved(false)
    if (!fields.check(event.currentTarget, { fullName: checkFullName(fullName) })) {
      return
    }

    setSubmitting(true)
    try {
      const updated = await updateFullName(fullName.trim())
      replaceAccount(updated)
      setFullName(updated.full_name ?? '')
      setSaved(true)
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="card" aria-labelledby="profile-title">
      <h2 id="profile-title">Nome</h2>
      <form className="form" onSubmit={handleSubmit} noValidate>
        <TextField
          label="Nome completo"
          name="fullName"
          autoComplete="name"
          maxLength={maxFullNameLength}
          value={fullName}
          error={fields.errors.fullName}
          onChange={(event) => {
            setFullName(event.target.value)
            setSaved(false)
            fields.clear('fullName')
          }}
          required
        />
        {error && <Alert tone="error">{error}</Alert>}
        {saved && <Alert tone="success">Nome atualizado.</Alert>}
        <div>
          <Button type="submit" loading={submitting}>
            Salvar nome
          </Button>
        </div>
      </form>
    </section>
  )
}

const emptyPasswords = { current: '', next: '', confirmation: '' }

function PasswordCard() {
  const [passwords, setPasswords] = useState(emptyPasswords)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [changed, setChanged] = useState(false)
  const fields = useFieldErrors<'currentPassword' | 'newPassword' | 'newPasswordConfirmation'>()

  function update(field: keyof typeof emptyPasswords, errorField: 'currentPassword' | 'newPassword' | 'newPasswordConfirmation', value: string) {
    setPasswords((current) => ({ ...current, [field]: value }))
    setChanged(false)
    fields.clear(errorField)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setChanged(false)
    const valid = fields.check(event.currentTarget, {
      currentPassword: checkRequired(passwords.current, 'Informe sua senha atual.'),
      newPassword:
        checkNewPassword(passwords.next) ??
        (passwords.next === passwords.current ? 'A nova senha precisa ser diferente da atual.' : undefined),
      newPasswordConfirmation: checkConfirmation(passwords.next, passwords.confirmation),
    })
    if (!valid) {
      return
    }

    setSubmitting(true)
    try {
      await changePassword(passwords.current, passwords.next, passwords.confirmation)
      setPasswords(emptyPasswords)
      setChanged(true)
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="card" aria-labelledby="password-title">
      <h2 id="password-title">Senha</h2>
      <p className="card__lead">Ao trocar a senha, você continua conectado aqui e sai de todos os outros aparelhos.</p>
      <form className="form" onSubmit={handleSubmit} noValidate>
        <TextField
          label="Senha atual"
          name="currentPassword"
          type="password"
          autoComplete="current-password"
          value={passwords.current}
          error={fields.errors.currentPassword}
          onChange={(event) => update('current', 'currentPassword', event.target.value)}
          required
        />
        <TextField
          label="Nova senha"
          name="newPassword"
          type="password"
          autoComplete="new-password"
          hint={`Mínimo de ${minPasswordLength} caracteres.`}
          value={passwords.next}
          error={fields.errors.newPassword}
          onChange={(event) => update('next', 'newPassword', event.target.value)}
          required
        />
        <TextField
          label="Confirme a nova senha"
          name="newPasswordConfirmation"
          type="password"
          autoComplete="new-password"
          value={passwords.confirmation}
          error={fields.errors.newPasswordConfirmation}
          onChange={(event) => update('confirmation', 'newPasswordConfirmation', event.target.value)}
          required
        />
        {error && <Alert tone="error">{error}</Alert>}
        {changed && <Alert tone="success">Senha alterada. Os outros aparelhos foram desconectados.</Alert>}
        <div>
          <Button type="submit" loading={submitting}>
            Trocar senha
          </Button>
        </div>
      </form>
    </section>
  )
}

export function AccountPage() {
  const { state } = useAuth()

  if (state.status !== 'authenticated') {
    return <PageLoader />
  }

  const account = state.user

  return (
    <div className="container page">
      <header className="page__header">
        <h1>Minha conta</h1>
        <p>Veja seus dados, ajuste seu nome e troque a senha.</p>
      </header>

      <div className="account">
        <section className="card" aria-labelledby="info-title">
          <h2 id="info-title">Informações</h2>
          <dl className="facts">
            <div>
              <dt>Nome</dt>
              <dd>{account.full_name ?? <span className="muted">Não informado</span>}</dd>
            </div>
            <div>
              <dt>Email</dt>
              <dd>{account.email}</dd>
            </div>
            <div>
              <dt>Formas de entrar</dt>
              <dd>
                <SignInMethods account={account} />
              </dd>
            </div>
            <div>
              <dt>Conta criada em</dt>
              <dd>{formatLongDate(account.created_at)}</dd>
            </div>
          </dl>
        </section>

        <ProfileCard account={account} />

        {account.has_password ? (
          <PasswordCard />
        ) : (
          <section className="card" aria-labelledby="password-title">
            <h2 id="password-title">Senha</h2>
            <p className="card__lead">
              Sua conta entra com o Google, então não há senha do ShortBit para trocar. Para mudar a senha, altere-a na sua
              Conta Google.
            </p>
          </section>
        )}
      </div>
    </div>
  )
}
