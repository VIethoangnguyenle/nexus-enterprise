import { useRef, useState } from 'react'
import { Building2, Check, ChevronDown } from 'lucide-react'
import { Popover } from '../primitives'
import type { DepartmentCount } from './contacts-model'

/**
 * "Phòng ban" filter: a chip that opens the departments people belong to, each
 * with its head count. The first entry clears it. Departments are picked by
 * name; there is nothing to type.
 */
export function DepartmentFilter({ departments, value, onChange }: {
  departments: DepartmentCount[]
  value: string
  onChange: (department: string) => void
}) {
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)

  const option = (name: string, label: string, count?: number) => (
    // A row of a listbox (role=option) with a tick for the chosen one; no primitive models an option,
    // and MenuItem is a menuitem, not a selectable option.
    // eslint-disable-next-line no-restricted-syntax
    <button
      key={name || 'all'}
      type="button"
      role="option"
      aria-selected={value === name}
      onClick={() => { onChange(name); setOpen(false) }}
      className="w-full flex items-center gap-2.5 h-9 px-2.5 rounded-md border-none bg-transparent cursor-pointer
        text-sm text-left text-ink focus-ring hover:bg-hover transition-colors duration-quick"
    >
      <span className="flex-1 truncate">{label}</span>
      {count !== undefined && <span className="text-xs text-ink-muted tnum">{count}</span>}
      {value === name && <Check size={16} strokeWidth={1.75} className="text-accent shrink-0" aria-hidden="true" />}
    </button>
  )

  return (
    <>
      {/* eslint-disable-next-line no-restricted-syntax -- A filter chip that opens a list: FilterChip is a toggle
          (aria-pressed) with no popup semantics or chevron. */}
      <button
        ref={anchor}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className={`press inline-flex items-center gap-1.5 h-8 px-3 rounded-md border-none cursor-pointer text-small focus-ring
          ${value ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover'}`}
      >
        <Building2 size={16} strokeWidth={1.75} aria-hidden="true" />
        {value ? `Phòng ban: ${value}` : 'Phòng ban'}
        <ChevronDown size={14} strokeWidth={1.75} aria-hidden="true" />
      </button>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={anchor} label="Lọc theo phòng ban" className="min-w-64 max-h-80 overflow-y-auto">
        <div role="listbox" aria-label="Phòng ban" className="grid">
          {option('', 'Tất cả phòng ban')}
          {departments.map((d) => option(d.name, d.name, d.count))}
        </div>
      </Popover>
    </>
  )
}
