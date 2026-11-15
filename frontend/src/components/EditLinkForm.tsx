import { useId, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { messageOf } from '../api/client.ts'
import { updateLink, type LinkChanges } from '../api/links.ts'
import type { ShortLink, Visibility } from '../api/types.ts'
import { useFieldErrors } from '../useFieldErrors.ts'
import { checkLinkPassword, checkURL } from '../validation.ts'
import { Alert } from './Alert.tsx'
import { Button } from './Button.tsx'
import { PasswordStrength } from './PasswordStrength.tsx'
import { TextField } from './TextField.tsx'
import { VisibilityToggle } from './VisibilityToggle.tsx'

type EditLinkFormProps = {
  link: ShortLink
  onSaved: (link: ShortLink) => void
  onCancel: () => void
}

// Edits a link's destination, visibility and password, sending only what
// changed. On a link that is already private, a blank password keeps the
// current one; a new one signs out everyone who unlocked it before.
export function EditLinkForm({ link, onSaved, onCancel }: EditLinkFormProps) {
  const [url, setUrl] = useState(link.original_url)
  const [visibility, setVisibility] = useState<Visibility>(link.visibility)
  const [password, setPassword] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const fields = useFieldErrors<'url' | 'linkPassword'>()
  const strengthId = useId()

  const wasPrivate = link.visibility === 'private'
  const changes: LinkChanges = {}
  if (url.trim() !== link.original_url) {
    changes.url = url.trim()
  }
  if (visibility !== link.visibility) {
    changes.visibility = visibility
  }
  if (visibility === 'private' && password !== '') {
    changes.password = password
  }
  const hasChanges = Object.keys(changes).length > 0

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const needsPassword = visibility === 'private' && (!wasPrivate || password !== '')
    const valid = fields.check(event.currentTarget, {
      url: checkURL(url),
      linkPassword: needsPassword ? checkLinkPassword(password) : undefined,
    })
    if (!valid || !hasChanges) {
      return
    }

    setSaving(true)
    try {
      const updated = await updateLink(link.short_code, changes)
      toast.success('Link atualizado')
      onSaved(updated)
    } catch (saveError) {
      setError(messageOf(saveError))
      setSaving(false)
    }
  }

  return (
    <form className="grid gap-5" onSubmit={handleSubmit} noValidate aria-label="Editar link">
      <TextField
        label="Destino"
        name="url"
        type="url"
        inputMode="url"
        value={url}
        error={fields.errors.url}
        onChange={(event) => {
          setUrl(event.target.value)
          fields.clear('url')
        }}
        required
      />

      <VisibilityToggle
        value={visibility}
        onChange={(value) => {
          setVisibility(value)
          fields.clear('linkPassword')
        }}
      />

      {visibility === 'private' && (
        <div className="grid gap-2.5 animate-in fade-in slide-in-from-top-2 duration-200">
          <TextField
            label={wasPrivate ? 'Nova senha do link' : 'Senha do link'}
            name="linkPassword"
            type="password"
            autoComplete="new-password"
            hint={
              wasPrivate
                ? 'Deixe em branco para manter a senha atual. Uma senha nova desconecta quem já abriu o link.'
                : 'Quem abrir o link vai precisar dessa senha.'
            }
            describedBy={strengthId}
            value={password}
            error={fields.errors.linkPassword}
            onChange={(event) => {
              setPassword(event.target.value)
              fields.clear('linkPassword')
            }}
          />
          {password !== '' && <PasswordStrength id={strengthId} password={password} />}
        </div>
      )}

      {error && <Alert tone="error">{error}</Alert>}

      <div className="flex flex-wrap gap-3">
        <Button type="submit" loading={saving} disabled={!hasChanges}>
          Salvar alterações
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={saving}>
          Cancelar
        </Button>
      </div>
    </form>
  )
}
