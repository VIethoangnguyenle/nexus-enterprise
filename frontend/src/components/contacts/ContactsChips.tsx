import type { ContactsCat } from '../../lib/contacts-search'
import { Pressable } from '../primitives'
import type { CategoryDef } from './contacts-model'

/**
 * The categories of Danh bạ on a tablet, where the list panel is too wide to keep: one chip each, in the
 * same order, below the title (DESIGN.md "Danh bạ"). A row of chips that grows too long scrolls inside itself.
 */
export function ContactsChips({ categories, current, onOpen, myDepartment, mineIsCurrent, onOpenMine }: {
  categories: CategoryDef[]
  current: ContactsCat
  onOpen: (cat: CategoryDef['id']) => void
  /** The shortcut to the caller's own department, as the list panel and the phone's first screen have it. */
  myDepartment?: { id: string; name: string }
  mineIsCurrent?: boolean
  onOpenMine?: () => void
}) {
  return (
    <div role="group" aria-label="Danh mục" className="flex gap-2 px-4 py-1 overflow-x-auto [scrollbar-width:none]">
      {categories.map((c) => (
        <Pressable
          key={c.id}
          aria-pressed={current === c.id && !mineIsCurrent}
          onClick={() => onOpen(c.id)}
          className={`press shrink-0 h-9 px-3 rounded-lg whitespace-nowrap text-small-ui transition-colors duration-quick
            ${current === c.id && !mineIsCurrent ? 'bg-accent-wash font-semibold text-ink' : 'bg-sunk font-medium text-ink hover:bg-hover'}`}
        >
          {c.title}
        </Pressable>
      ))}
      {myDepartment && onOpenMine && (
        <Pressable
          aria-pressed={!!mineIsCurrent}
          onClick={onOpenMine}
          className={`press shrink-0 h-9 px-3 rounded-lg whitespace-nowrap text-small-ui transition-colors duration-quick
            ${mineIsCurrent ? 'bg-accent-wash font-semibold text-ink' : 'bg-sunk font-medium text-ink hover:bg-hover'}`}
        >
          Phòng của tôi
        </Pressable>
      )}
    </div>
  )
}
