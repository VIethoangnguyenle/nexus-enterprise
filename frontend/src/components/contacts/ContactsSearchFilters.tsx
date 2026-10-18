import { useMemo, useState } from 'react'
import { ChevronDown, X } from 'lucide-react'
import { FilterChip, IconButton, Pressable } from '../primitives'
import { NavSheet, SheetItem, SheetSeparator } from '../spaces/NavSheet'
import type { DirectoryTree, RoleGroup } from './contacts-model'
import { deptChoices, type PlaceFilter } from './place-filter'

interface ContactsSearchFiltersProps {
  isPersonal: boolean
  filter: PlaceFilter | undefined
  onFilter: (next: PlaceFilter | undefined) => void
  /** The departments the viewer may see; undefined until they are loaded. */
  tree: DirectoryTree | undefined
  roles: RoleGroup[]
  onlineOnly: boolean
  onToggleOnline: () => void
  /** How many of what is listed are online; shown once known. */
  onlineCount?: number
}

type Picker = 'dept' | 'role' | null

/** A chip that picks a place from a sheet; one without `onOpen` only shows the place it was opened with, and clears it. */
function PickChip({ name, chosen, onOpen, onClear }: { name: string; chosen?: string; onOpen?: () => void; onClear: () => void }) {
  if (chosen === undefined && onOpen) {
    return (
      <Pressable
        onClick={onOpen}
        aria-haspopup="dialog"
        className="shrink-0 inline-flex items-center gap-1 h-9 pl-3 pr-2 rounded-full text-sm font-medium text-ink ring-1 ring-inset ring-line
          active:bg-hover focus-ring"
      >
        {name}
        <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
      </Pressable>
    )
  }
  return (
    <span className="shrink-0 inline-flex items-center h-9 max-w-64 rounded-full bg-accent-wash ring-1 ring-inset ring-accent">
      {onOpen ? (
        <Pressable onClick={onOpen} aria-haspopup="dialog" aria-label={`${name}: ${chosen}`}
          className="h-9 min-w-0 pl-3 pr-1 rounded-l-full text-sm font-semibold text-ink truncate focus-ring">
          {chosen}
        </Pressable>
      ) : (
        <span className="min-w-0 pl-3 pr-1 text-sm font-semibold text-ink truncate">{name}: {chosen}</span>
      )}
      <IconButton type="button" shape="round" aria-label={`Bỏ lọc ${name}: ${chosen}`} className="w-9 h-9 shrink-0" onClick={onClear}>
        <X size={16} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
    </span>
  )
}

/**
 * The filter chips of a phone's people search (DESIGN.md "Danh bạ" › Điện thoại, "Tìm người"): Phòng ban · Vai trò in an
 * organization (a personal workspace has neither), the group the search was opened from, and Đang trực tuyến. A place is
 * picked from a sheet of names, never typed; one place at a time.
 */
export function ContactsSearchFilters(p: ContactsSearchFiltersProps) {
  const [picker, setPicker] = useState<Picker>(null)
  const depts = useMemo(() => (p.tree ? deptChoices(p.tree) : []), [p.tree])
  const of = (kind: PlaceFilter['kind']) => (p.filter?.kind === kind ? p.filter.label : undefined)
  const choose = (kind: 'dept' | 'role', id: string, label: string) => {
    setPicker(null)
    p.onFilter({ kind, id, label })
  }
  const options = picker === 'dept' ? depts.map((d) => ({ id: d.id, label: d.label })) : p.roles.map((r) => ({ id: r.id, label: r.name }))

  return (
    <div role="group" aria-label="Bộ lọc" className="flex items-center gap-2 min-w-0 overflow-x-auto [scrollbar-width:none]">
      {p.filter?.kind === 'group' && (
        <PickChip name="Nhóm" chosen={p.filter.label} onClear={() => p.onFilter(undefined)} />
      )}
      {!p.isPersonal && (
        <>
          <PickChip name="Phòng ban" chosen={of('dept')} onOpen={() => setPicker('dept')} onClear={() => p.onFilter(undefined)} />
          <PickChip name="Vai trò" chosen={of('role')} onOpen={() => setPicker('role')} onClear={() => p.onFilter(undefined)} />
        </>
      )}
      <FilterChip pressed={p.onlineOnly} onClick={p.onToggleOnline}>
        <span className="inline-flex items-center gap-1.5">
          <span aria-hidden="true" className="w-2 h-2 rounded-full bg-success" />
          Đang trực tuyến{p.onlineCount !== undefined && <span className="font-normal text-ink-muted tnum">{p.onlineCount}</span>}
        </span>
      </FilterChip>

      {picker && (
        <NavSheet label={picker === 'dept' ? 'Phòng ban' : 'Vai trò'} onClose={() => setPicker(null)} roomy>
          <div role="radiogroup" aria-label={picker === 'dept' ? 'Phòng ban' : 'Vai trò'} className="grid gap-1">
            {options.map((o) => (
              <SheetItem key={o.id} checked={p.filter?.kind === picker && p.filter.id === o.id} onClick={() => choose(picker, o.id, o.label)}>
                {o.label}
              </SheetItem>
            ))}
            {options.length === 0 && <p className="m-0 px-3.5 py-3 text-sm text-ink-muted">Chưa có gì để chọn.</p>}
          </div>
          {p.filter?.kind === picker && (
            <>
              <SheetSeparator />
              <SheetItem onClick={() => { setPicker(null); p.onFilter(undefined) }}>Bỏ lọc</SheetItem>
            </>
          )}
        </NavSheet>
      )}
    </div>
  )
}
