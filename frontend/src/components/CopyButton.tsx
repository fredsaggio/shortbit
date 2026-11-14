import { useEffect, useState } from 'react'
import { Button } from './Button.tsx'

const resetDelayMs = 2000

export function CopyButton({ text, label = 'Copiar', small = false }: { text: string; label?: string; small?: boolean }) {
  const [status, setStatus] = useState<'idle' | 'copied' | 'failed'>('idle')

  useEffect(() => {
    if (status === 'idle') {
      return
    }
    const timer = window.setTimeout(() => setStatus('idle'), resetDelayMs)
    return () => window.clearTimeout(timer)
  }, [status])

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      setStatus('copied')
    } catch {
      setStatus('failed')
    }
  }

  return (
    <Button type="button" variant="secondary" small={small} onClick={copy} aria-live="polite">
      {status === 'copied' ? 'Copiado!' : status === 'failed' ? 'Não deu para copiar' : label}
    </Button>
  )
}
