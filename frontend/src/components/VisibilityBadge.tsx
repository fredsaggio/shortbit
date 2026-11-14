import { Globe, Lock } from 'lucide-react'
import type { Visibility } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'

export function VisibilityBadge({ visibility }: { visibility: Visibility }) {
  return visibility === 'private' ? (
    <Badge variant="outline" className="gap-1 border-brand/30 bg-brand-100 text-brand-700">
      <Lock className="size-3" aria-hidden="true" />
      Privado
    </Badge>
  ) : (
    <Badge variant="outline" className="gap-1 border-action/20 bg-navy-100 text-navy">
      <Globe className="size-3" aria-hidden="true" />
      Público
    </Badge>
  )
}
