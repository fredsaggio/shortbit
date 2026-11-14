import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router'
import { buttonSizes, buttonTones } from '../components/buttonStyles.ts'
import { cn } from '@/lib/utils'

export function NotFoundPage() {
  return (
    <section className="mx-auto grid max-w-xl justify-items-center gap-4 px-4 py-24 text-center">
      <p className="text-7xl font-extrabold tracking-tight text-brand-700">404</p>
      <h1 className="text-3xl font-extrabold">Página não encontrada</h1>
      <p className="text-muted-foreground">O endereço que você abriu não existe aqui.</p>
      <Link to="/" className={cn('mt-2 inline-flex items-center gap-2 rounded-md font-semibold transition-colors', buttonSizes.md, buttonTones.primary)}>
        <ArrowLeft className="size-4" aria-hidden="true" />
        Voltar para o início
      </Link>
    </section>
  )
}
