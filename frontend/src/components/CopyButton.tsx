import { Check, Copy } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from './Button.tsx'

const resetDelayMs = 2000

type CopyButtonProps = {
  text: string
  label?: string
  size?: 'sm' | 'md'
  variant?: 'outline' | 'navy' | 'onDarkOutline'
}

export function CopyButton({ text, label = 'Copiar', size = 'md', variant = 'outline' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) {
      return
    }
    const timer = window.setTimeout(() => setCopied(false), resetDelayMs)
    return () => window.clearTimeout(timer)
  }, [copied])

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      toast.success('Link copiado', { description: text })
    } catch {
      toast.error('Não foi possível copiar. Selecione o link e copie manualmente.')
    }
  }

  return (
    <Button type="button" variant={variant} size={size} onClick={copy} aria-live="polite">
      {copied ? <Check className="size-4" aria-hidden="true" /> : <Copy className="size-4" aria-hidden="true" />}
      {copied ? 'Copiado!' : label}
    </Button>
  )
}
