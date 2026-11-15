import type { ReactNode } from 'react'
import { useState } from 'react'
import { messageOf } from '../api/client.ts'
import { Alert } from './Alert.tsx'
import { Button } from './Button.tsx'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

type ConfirmDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: ReactNode
  confirmLabel: string
  busyLabel: string
  /** Runs the action; a thrown error is shown inside the dialog. */
  onConfirm: () => Promise<void>
  /** Extra fields, such as the password that confirms an account deletion. */
  children?: ReactNode
  confirmDisabled?: boolean
}

// A confirmation for destructive actions. It stays open while the request
// runs, so a slow or failed request never looks like it worked.
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  busyLabel,
  onConfirm,
  children,
  confirmDisabled = false,
}: ConfirmDialogProps) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function confirm() {
    setBusy(true)
    setError('')
    try {
      await onConfirm()
    } catch (confirmError) {
      setError(messageOf(confirmError))
      setBusy(false)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!busy) {
          setError('')
          onOpenChange(next)
        }
      }}
    >
      <AlertDialogContent className="rounded-2xl">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-xl font-bold text-navy">{title}</AlertDialogTitle>
          <AlertDialogDescription className="text-[15px] text-muted-foreground">{description}</AlertDialogDescription>
        </AlertDialogHeader>
        {children}
        {error && <Alert tone="error">{error}</Alert>}
        <AlertDialogFooter className="gap-2">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancelar
          </Button>
          <Button type="button" variant="danger" onClick={confirm} loading={busy} disabled={confirmDisabled}>
            {busy ? busyLabel : confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
