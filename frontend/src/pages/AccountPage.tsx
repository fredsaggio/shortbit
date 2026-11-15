import { KeyRound, Mail, ShieldCheck, Trash2, UserRound } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'
import { changePassword, deleteAccount, updateFullName } from '../api/auth.ts'
import { messageOf } from '../api/client.ts'
import type { Account } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Alert } from '../components/Alert.tsx'
import { Button } from '../components/Button.tsx'
import { ConfirmDialog } from '../components/ConfirmDialog.tsx'
import { PageLoader } from '../components/PageLoader.tsx'
import { PasswordChecklist } from '../components/PasswordChecklist.tsx'
import { TextField } from '../components/TextField.tsx'
import { formatLongDate } from '../format.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkConfirmation, checkFullName, checkNewPassword, checkRequired, maxFullNameLength, passwordRules } from '../validation.ts'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const tabTriggerClass =
  'h-10 flex-1 gap-2 rounded-md sm:flex-none px-4 text-[15px] font-semibold text-muted-foreground transition-colors hover:bg-card/70 hover:text-navy data-[state=active]:bg-card data-[state=active]:text-navy data-[state=active]:shadow-card'

function SignInMethods({ account }: { account: Account }) {
  return (
    <span className="flex flex-wrap gap-1.5">
      {account.has_password && (
        <Badge variant="outline" className="gap-1 border-action/20 bg-navy-100 text-navy">
          <KeyRound className="size-3" aria-hidden="true" />
          Email e senha
        </Badge>
      )}
      {account.has_google && (
        <Badge variant="outline" className="gap-1 bg-card text-navy">
          Google
        </Badge>
      )}
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
      toast.success('Nome atualizado')
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section aria-labelledby="profile-title" className="rounded-3xl border bg-card p-6 shadow-card sm:p-8">
      <h2 id="profile-title" className="text-xl font-bold">
        Nome
      </h2>
      <p className="mt-1 text-muted-foreground">É assim que você aparece no shortbit.</p>
      <form className="mt-6 grid gap-5" onSubmit={handleSubmit} noValidate>
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
type PasswordField = 'currentPassword' | 'newPassword' | 'newPasswordConfirmation'

function PasswordCard() {
  const [passwords, setPasswords] = useState(emptyPasswords)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [changed, setChanged] = useState(false)
  const fields = useFieldErrors<PasswordField>()
  const checklistId = useId()

  function update(field: keyof typeof emptyPasswords, errorField: PasswordField, value: string) {
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
      toast.success('Senha alterada', { description: 'Os outros aparelhos foram desconectados.' })
    } catch (submitError) {
      setError(messageOf(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section aria-labelledby="password-title" className="rounded-3xl border bg-card p-6 shadow-card sm:p-8">
      <h2 id="password-title" className="text-xl font-bold">
        Trocar senha
      </h2>
      <p className="mt-1 text-muted-foreground">Você continua conectado aqui e sai de todos os outros aparelhos.</p>
      <form className="mt-6 grid gap-5" onSubmit={handleSubmit} noValidate>
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
        <div className="grid gap-5 sm:grid-cols-2">
          <TextField
            label="Nova senha"
            name="newPassword"
            type="password"
            autoComplete="new-password"
            describedBy={checklistId}
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
            describedBy={checklistId}
            value={passwords.confirmation}
            error={fields.errors.newPasswordConfirmation}
            onChange={(event) => update('confirmation', 'newPasswordConfirmation', event.target.value)}
            required
          />
        </div>
        <PasswordChecklist
          id={checklistId}
          rules={passwordRules({ password: passwords.next, current: passwords.current })}
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

// Deleting the account asks for the password, or for the email when the
// account only signs in with Google, because an open session is not enough.
function DeleteAccountCard({ account }: { account: Account }) {
  const { refresh } = useAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [confirmation, setConfirmation] = useState('')
  const fieldId = useId()

  return (
    <section aria-labelledby="delete-account-title" className="mt-5 rounded-3xl border border-destructive/25 bg-card p-6 shadow-card sm:p-8">
      <h2 id="delete-account-title" className="text-xl font-bold text-destructive">
        Excluir conta
      </h2>
      <p className="mt-1 text-muted-foreground">
        Apaga sua conta e todos os seus links. Os links param de funcionar para quem abrir, e isso não pode ser desfeito.
      </p>
      <Button type="button" variant="danger" className="mt-5" onClick={() => setOpen(true)}>
        <Trash2 className="size-4" aria-hidden="true" />
        Excluir minha conta
      </Button>

      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          if (!next) {
            setConfirmation('')
          }
        }}
        title="Excluir sua conta?"
        description="Sua conta, seus links e a contagem de cliques serão apagados para sempre, e você sai de todos os aparelhos."
        confirmLabel="Excluir conta"
        busyLabel="Excluindo…"
        confirmDisabled={confirmation.trim() === ''}
        onConfirm={async () => {
          await deleteAccount(account.has_password ? { password: confirmation } : { email: confirmation })
          navigate('/', { replace: true })
          await refresh()
          toast.success('Sua conta foi excluída')
        }}
      >
        <TextField
          id={fieldId}
          label={account.has_password ? 'Digite sua senha para confirmar' : `Digite seu email (${account.email}) para confirmar`}
          name="deleteConfirmation"
          type={account.has_password ? 'password' : 'email'}
          autoComplete={account.has_password ? 'current-password' : 'off'}
          value={confirmation}
          onChange={(event) => setConfirmation(event.target.value)}
        />
      </ConfirmDialog>
    </section>
  )
}

export function AccountPage() {
  const { state } = useAuth()

  if (state.status !== 'authenticated') {
    return <PageLoader />
  }

  const account = state.user
  const initials = (account.full_name ?? account.email)
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, account.full_name ? 2 : 1)
    .map((part) => part[0])
    .join('')
    .toUpperCase()

  return (
    <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6 sm:py-12">
      <header>
        <h1 className="text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">Minha conta</h1>
        <p className="mt-1 text-muted-foreground">Veja seus dados, ajuste seu nome e troque a senha.</p>
      </header>

      <section aria-labelledby="info-title" className="mt-8 rounded-3xl border bg-card p-6 shadow-card sm:p-8">
        <h2 id="info-title" className="sr-only">
          Informações
        </h2>
        <div className="flex items-center gap-4">
          <Avatar className="size-16">
            <AvatarFallback className="bg-brand text-xl font-extrabold text-white">{initials}</AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="truncate text-xl font-bold">{account.full_name ?? 'Sem nome cadastrado'}</p>
            <p className="truncate text-muted-foreground">{account.email}</p>
          </div>
        </div>
        <dl className="mt-6 grid gap-5 border-t pt-6 sm:grid-cols-3">
          <div>
            <dt className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <UserRound className="size-4" aria-hidden="true" /> Nome
            </dt>
            <dd className="mt-1 font-semibold">{account.full_name ?? <span className="font-normal text-muted-foreground">Não informado</span>}</dd>
          </div>
          <div>
            <dt className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <ShieldCheck className="size-4" aria-hidden="true" /> Formas de entrar
            </dt>
            <dd className="mt-1.5">
              <SignInMethods account={account} />
            </dd>
          </div>
          <div>
            <dt className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <Mail className="size-4" aria-hidden="true" /> Conta criada em
            </dt>
            <dd className="mt-1 font-semibold">{formatLongDate(account.created_at)}</dd>
          </div>
        </dl>
      </section>

      <Tabs defaultValue="perfil" className="mt-8 gap-5">
        <TabsList className="h-auto w-full justify-start gap-1 rounded-lg bg-surface-200 p-1 sm:w-fit sm:self-start">
          <TabsTrigger value="perfil" className={tabTriggerClass}>
            <UserRound className="size-4" aria-hidden="true" />
            Perfil
          </TabsTrigger>
          <TabsTrigger value="seguranca" className={tabTriggerClass}>
            <KeyRound className="size-4" aria-hidden="true" />
            Segurança
          </TabsTrigger>
        </TabsList>
        <TabsContent value="perfil" className="animate-in fade-in duration-200">
          <ProfileCard account={account} />
        </TabsContent>
        <TabsContent value="seguranca" className="animate-in fade-in duration-200">
          {account.has_password ? (
            <PasswordCard />
          ) : (
            <section aria-labelledby="password-title" className="rounded-3xl border bg-card p-6 shadow-card sm:p-8">
              <h2 id="password-title" className="text-xl font-bold">
                Senha
              </h2>
              <p className="mt-2 text-muted-foreground">
                Sua conta entra com o Google, então não há senha do shortbit para trocar. Para mudar a senha, altere-a na sua
                Conta Google.
              </p>
            </section>
          )}
          <DeleteAccountCard account={account} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
