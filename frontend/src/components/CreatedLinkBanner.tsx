import { CircleCheck, QrCode } from 'lucide-react'
import { useState } from 'react'
import type { CreatedLink } from '../api/types.ts'
import { Button } from './Button.tsx'
import { CopyButton } from './CopyButton.tsx'
import { QrCodeDialog } from './QrCodeDialog.tsx'
import { cn } from '@/lib/utils'

type CreatedLinkBannerProps = {
  link: CreatedLink
  className?: string
  /** Opens the QR code right away, for someone who asked for it. */
  openQrCode?: boolean
}

// The link just created, ready to copy or to turn into a QR code.
export function CreatedLinkBanner({ link, className, openQrCode = false }: CreatedLinkBannerProps) {
  const [showingQrCode, setShowingQrCode] = useState(openQrCode)

  return (
    <div
      role="status"
      className={cn(
        'flex flex-wrap items-center gap-3 rounded-xl border border-success/25 bg-success-soft p-4 animate-in fade-in zoom-in-95 duration-200',
        className,
      )}
    >
      <CircleCheck className="size-5 text-success" aria-hidden="true" />
      <a href={link.short_url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate font-bold text-action hover:underline">
        {link.short_url}
      </a>
      <div className="flex gap-2">
        <CopyButton text={link.short_url} />
        <Button type="button" variant="outline" onClick={() => setShowingQrCode(true)}>
          <QrCode className="size-4" aria-hidden="true" />
          QR code
        </Button>
      </div>
      <QrCodeDialog open={showingQrCode} onOpenChange={setShowingQrCode} shortCode={link.short_code} />
    </div>
  )
}
