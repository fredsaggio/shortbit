import { Globe, Lock } from 'lucide-react'
import { useId } from 'react'
import type { Visibility } from '../api/types.ts'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const itemClass =
  'h-11 flex-1 gap-2 rounded-md px-4 text-[15px] font-semibold text-muted-foreground transition-colors hover:bg-surface hover:text-navy data-[state=on]:bg-navy data-[state=on]:text-white data-[state=on]:shadow-sm sm:flex-none'

type VisibilityToggleProps = {
  value: Visibility
  onChange: (value: Visibility) => void
}

// Segmented control for who can open a link, shared by the home page and the
// dashboard. Radix renders it as a radio group.
export function VisibilityToggle({ value, onChange }: VisibilityToggleProps) {
  const labelId = useId()

  return (
    <div className="grid gap-2">
      <span id={labelId} className="text-sm font-semibold text-navy">
        Quem pode abrir
      </span>
      <ToggleGroup
        type="single"
        value={value}
        onValueChange={(next) => {
          if (next) {
            onChange(next as Visibility)
          }
        }}
        aria-labelledby={labelId}
        className="flex w-full gap-1 rounded-lg border bg-surface p-1 sm:w-fit sm:justify-self-start"
      >
        <ToggleGroupItem value="public" className={itemClass}>
          <Globe className="size-4" aria-hidden="true" />
          Público
        </ToggleGroupItem>
        <ToggleGroupItem value="private" className={itemClass}>
          <Lock className="size-4" aria-hidden="true" />
          Com senha
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}
