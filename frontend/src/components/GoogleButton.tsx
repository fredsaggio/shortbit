import { googleLoginPath } from '../api/auth.ts'
import { cn } from '@/lib/utils'
import { buttonSizes, buttonTones } from './buttonStyles.ts'

export function GoogleButton({ label = 'Continuar com Google' }: { label?: string }) {
  return (
    <a
      href={googleLoginPath}
      className={cn(
        'inline-flex w-full items-center justify-center gap-3 rounded-md border font-semibold transition-[background-color,border-color] duration-150',
        buttonSizes.md,
        buttonTones.outline,
      )}
    >
      <svg viewBox="0 0 24 24" className="size-5" aria-hidden="true">
        <path fill="#4285F4" d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.4h6.5a5.6 5.6 0 0 1-2.4 3.6v3h3.9c2.2-2.1 3.5-5.1 3.5-8.7z" />
        <path fill="#34A853" d="M12 24c3.2 0 6-1.1 7.9-2.9l-3.9-3c-1.1.7-2.4 1.1-4 1.1-3.1 0-5.7-2.1-6.6-4.9h-4v3.1A12 12 0 0 0 12 24z" />
        <path fill="#FBBC05" d="M5.4 14.3a7.2 7.2 0 0 1 0-4.6V6.6h-4a12 12 0 0 0 0 10.8l4-3.1z" />
        <path fill="#EA4335" d="M12 4.8c1.8 0 3.3.6 4.6 1.8l3.4-3.4A12 12 0 0 0 1.4 6.6l4 3.1C6.3 6.9 8.9 4.8 12 4.8z" />
      </svg>
      {label}
    </a>
  )
}
