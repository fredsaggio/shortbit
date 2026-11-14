import type { Visibility } from '../api/types.ts'

export function VisibilityBadge({ visibility }: { visibility: Visibility }) {
  return visibility === 'private' ? (
    <span className="badge badge--private">Com senha</span>
  ) : (
    <span className="badge badge--public">Público</span>
  )
}
