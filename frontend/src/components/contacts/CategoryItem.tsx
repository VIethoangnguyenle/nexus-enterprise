import { Building2, ChevronRight, Folder, Shield, Users, UsersRound, type LucideIcon } from 'lucide-react'
import { Pressable } from '../primitives'
import type { CategoryDef } from './contacts-model'

/** What the shortcut "Phòng của tôi" opens: the caller's own department. */
export interface MyDepartment {
  id: string
  name: string
  /** The path above it, once the departments are known. */
  path?: string
  /** People in it and beneath it, once the departments are known. */
  count?: number
}

const ICONS: Record<CategoryDef['id'], LucideIcon> = { org: Building2, roles: Shield, groups: UsersRound, members: Users }

interface CategoryItemProps {
  def: CategoryDef
  selected: boolean
  onOpen: () => void
  /** The phone's 64 high row with a chevron; the desktop's is 56. */
  tall?: boolean
}

/**
 * A category of Danh bạ (DESIGN.md "Danh bạ"): an icon square 40 on the wash of a person hue, the name
 * and one muted line of what is inside. The same on every width.
 */
export function CategoryItem({ def, selected, onOpen, tall }: CategoryItemProps) {
  const Icon = ICONS[def.id]
  return (
    <Pressable
      onClick={onOpen}
      aria-current={selected ? 'page' : undefined}
      className={`press flex items-center gap-3 w-full min-w-0 px-2.5 py-1.5 rounded-lg border-none text-left cursor-pointer
        focus-ring transition-colors duration-quick text-md ${tall ? 'min-h-16' : 'min-h-14'}
        ${selected ? 'bg-raised font-semibold' : 'bg-transparent hover:bg-hover'}`}
    >
      <span
        aria-hidden="true"
        className="grid place-items-center shrink-0 w-10 h-10 rounded-lg"
        style={{ background: `var(--color-person-${def.hue}-wash)`, color: `var(--color-person-${def.hue})` }}
      >
        <Icon size={20} strokeWidth={1.75} />
      </span>
      <span className="grid flex-1 min-w-0">
        <span className="truncate text-ink">{def.title}</span>
        <span className="truncate text-xs font-normal text-ink-muted">{def.sub}</span>
      </span>
      {tall && <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />}
    </Pressable>
  )
}

/** Phòng của tôi: the one shortcut with content, as it is the department people open most (like Lark's "My department"). */
export function MyDepartmentShortcut({ dept, selected, onOpen, tall }: {
  dept: MyDepartment; selected?: boolean; onOpen: () => void; tall?: boolean
}) {
  return (
    <>
      <span className="px-2.5 pt-3.5 pb-1 text-small font-semibold text-ink-muted">Phòng của tôi</span>
      <Pressable
        onClick={onOpen}
        aria-current={selected ? 'page' : undefined}
        className={`press flex items-center gap-3 w-full min-w-0 px-2.5 py-1.5 rounded-lg border-none text-left cursor-pointer
          focus-ring transition-colors duration-quick text-md ${tall ? 'min-h-16' : 'min-h-11'}
          ${selected ? 'bg-raised font-semibold' : 'bg-transparent hover:bg-hover'}`}
      >
        <span aria-hidden="true" className={`grid place-items-center shrink-0 rounded-md bg-sunk text-ink-muted ${tall ? 'w-10 h-10' : 'w-7 h-7'}`}>
          <Folder size={16} strokeWidth={1.75} />
        </span>
        <span className="grid flex-1 min-w-0">
          <span className="truncate text-ink">{dept.name}</span>
          {dept.path && <span className="truncate text-xs font-normal text-ink-muted">{dept.path}</span>}
        </span>
        {dept.count !== undefined && <span className="shrink-0 text-small font-medium text-ink-muted tnum">{dept.count}</span>}
        {tall && <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />}
      </Pressable>
    </>
  )
}
