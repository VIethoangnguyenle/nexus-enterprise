import { useRef, useState } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import type { AssetType } from '../../api/assets'
import { MenuItem, Popover, Pressable } from '../primitives'

interface TypeFilterProps {
  types: AssetType[]
  /** The chosen type's id, if any. */
  value: string | undefined
  onChange: (typeId: string | undefined) => void
}

/** "Loại: Tất cả" as a chip in the filter row (mockup §2); it opens a menu of type names. */
export function TypeFilter({ types, value, onChange }: TypeFilterProps) {
  const [open, setOpen] = useState(false)
  const anchor = useRef<HTMLButtonElement>(null)
  const chosen = types.find((t) => t.id === value)
  return (
    <>
      <Pressable
        ref={anchor}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Loại tài sản: ${chosen?.name ?? 'Tất cả'}`}
        onClick={() => setOpen((v) => !v)}
        className={`press inline-flex items-center gap-1.5 h-8 px-3 rounded-md text-small whitespace-nowrap
          ${chosen ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover'}`}
      >
        Loại: {chosen?.name ?? 'Tất cả'}
        <ChevronDown size={14} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
      </Pressable>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={anchor} role="menu" label="Chọn loại tài sản">
        <MenuItem onClick={() => onChange(undefined)} icon={!chosen ? <Check size={16} strokeWidth={1.75} aria-hidden="true" /> : <span className="w-4" />}>
          Tất cả
        </MenuItem>
        {types.map((t) => (
          <MenuItem key={t.id} onClick={() => onChange(t.id)} icon={t.id === value ? <Check size={16} strokeWidth={1.75} aria-hidden="true" /> : <span className="w-4" />}>
            {t.name}
          </MenuItem>
        ))}
      </Popover>
    </>
  )
}
