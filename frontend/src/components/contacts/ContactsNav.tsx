import type { ContactsCat } from '../../lib/contacts-search'
import { Heading } from '../primitives'
import { CategoryItem, MyDepartmentShortcut, type MyDepartment } from './CategoryItem'
import type { CategoryDef } from './contacts-model'
import { InlineAlert } from './ContactsStates'

interface ContactsNavProps {
  categories: CategoryDef[]
  current: ContactsCat
  onOpen: (cat: CategoryDef['id']) => void
  /** The caller's own department; absent for someone with none and in a personal workspace. */
  myDepartment?: MyDepartment
  /** The shortcut is the page being shown. */
  mineIsCurrent?: boolean
  onOpenMine?: () => void
  /** The departments failed to load: said under Phòng ban, the other categories keep working. */
  treeFailed?: boolean
  onRetryTree?: () => void
}

/**
 * The list panel of a desktop Danh bạ (DESIGN.md §5): 280 wide on the page tone, only the categories and
 * the shortcut. Nothing is listed here (no tree, no roles, no groups, no people): the content of a category
 * appears in the next column once it is opened.
 */
export function ContactsNav({
  categories, current, onOpen, myDepartment, mineIsCurrent, onOpenMine, treeFailed, onRetryTree,
}: ContactsNavProps) {
  return (
    <aside className="flex flex-col shrink-0 w-70 min-h-0 bg-base">
      <div className="px-4 pt-4 pb-2">
        <Heading as="h1" look="panel">Danh bạ</Heading>
      </div>
      <nav aria-label="Danh mục" className="grid content-start gap-0.5 min-h-0 overflow-y-auto px-2 pb-4">
        {categories.map((def) => (
          <div key={def.id} className="grid gap-1">
            <CategoryItem def={def} selected={current === def.id && !mineIsCurrent} onOpen={() => onOpen(def.id)} />
            {def.id === 'org' && treeFailed && onRetryTree && (
              <div className="px-1"><InlineAlert text="Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." onRetry={onRetryTree} /></div>
            )}
          </div>
        ))}
        {myDepartment && onOpenMine && (
          <MyDepartmentShortcut dept={myDepartment} selected={mineIsCurrent} onOpen={onOpenMine} />
        )}
      </nav>
    </aside>
  )
}
