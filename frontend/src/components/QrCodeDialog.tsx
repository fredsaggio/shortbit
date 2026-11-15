import { Download } from 'lucide-react'
import { useEffect, useState } from 'react'
import { shortLinkURL } from '../api/links.ts'
import { downloadFile, qrCodePNG, qrCodeSVG, qrCodeTarget, svgDataURL } from '../qrCode.ts'
import { Alert } from './Alert.tsx'
import { Button } from './Button.tsx'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'

type QrCodeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  shortCode: string
}

type QrState = { status: 'loading' } | { status: 'ready'; svg: string } | { status: 'error' }

const failureMessage = 'Não foi possível gerar o QR code. Tente de novo.'

export function QrCodeDialog({ open, onOpenChange, shortCode }: QrCodeDialogProps) {
  const url = shortLinkURL(shortCode)
  const displayURL = url.replace(/^https?:\/\//, '')
  const target = qrCodeTarget(url)
  const [state, setState] = useState<QrState>({ status: 'loading' })
  const [downloadError, setDownloadError] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    let current = true
    qrCodeSVG(target).then(
      (svg) => current && setState({ status: 'ready', svg }),
      () => current && setState({ status: 'error' }),
    )
    return () => {
      current = false
    }
  }, [open, target])

  async function downloadPNG() {
    setDownloadError(false)
    try {
      downloadFile(await qrCodePNG(target), `qrcode-${shortCode}.png`)
    } catch {
      setDownloadError(true)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setDownloadError(false)
        onOpenChange(next)
      }}
    >
      <DialogContent className="gap-5 rounded-2xl p-6 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-xl font-bold text-navy">QR code do link</DialogTitle>
          <DialogDescription className="text-[15px] text-muted-foreground">
            Abre <strong className="font-semibold break-all text-navy">{displayURL}</strong>. Se você mudar o destino, o QR code já
            impresso continua valendo.
          </DialogDescription>
        </DialogHeader>

        <div className="mx-auto w-full max-w-64">
          {state.status === 'loading' && <Skeleton className="aspect-square w-full rounded-xl" role="status" aria-label="Gerando o QR code…" />}
          {state.status === 'ready' && (
            <img src={svgDataURL(state.svg)} alt={`QR code de ${displayURL}`} className="aspect-square w-full rounded-xl border" />
          )}
          {state.status === 'error' && <Alert tone="error">{failureMessage}</Alert>}
        </div>

        {downloadError && <Alert tone="error">{failureMessage}</Alert>}

        <div className="grid grid-cols-2 gap-3">
          <Button type="button" variant="navy" onClick={downloadPNG} disabled={state.status !== 'ready'}>
            <Download className="size-4" aria-hidden="true" />
            Baixar PNG
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={state.status !== 'ready'}
            onClick={() => state.status === 'ready' && downloadFile(svgDataURL(state.svg), `qrcode-${shortCode}.svg`)}
          >
            <Download className="size-4" aria-hidden="true" />
            Baixar SVG
          </Button>
        </div>
        <p className="-mt-2 text-center text-sm text-muted-foreground">PNG para enviar por mensagem; SVG para impressão em qualquer tamanho.</p>
      </DialogContent>
    </Dialog>
  )
}
