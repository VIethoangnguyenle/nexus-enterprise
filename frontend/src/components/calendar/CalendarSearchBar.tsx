import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { IconButton, SearchField } from '../primitives'

interface Props {
  value: string
  onChange: (v: string) => void
  onClose: () => void
  /** On a phone the bar sits under the module bar with the same margins. */
  phone?: boolean
}

/** The pill "Tìm sự kiện" that replaces the top bar while searching. Esc or × closes it. */
export function CalendarSearchBar({ value, onChange, onClose, phone }: Props) {
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => ref.current?.focus(), [])
  return (
    <div className={phone ? 'px-3 pt-1 pb-2' : 'px-4 pt-3 pb-2'} role="search">
      <SearchField
        ref={ref}
        pill
        label="Tìm sự kiện"
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            e.preventDefault()
            onClose()
          }
        }}
        trailing={
          <IconButton size="lg" shape="round" aria-label="Đóng tìm kiếm" onClick={onClose}>
            <X size={18} strokeWidth={1.75} />
          </IconButton>
        }
      />
    </div>
  )
}
