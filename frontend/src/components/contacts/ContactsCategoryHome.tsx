import { Pressable } from '../primitives'
import { CategoryItem, MyDepartmentShortcut, type MyDepartment } from './CategoryItem'
import type { CategoryDef } from './contacts-model'

interface ContactsCategoryHomeProps {
  categories: CategoryDef[]
  onOpen: (cat: CategoryDef['id']) => void
  myDepartment?: MyDepartment
  onOpenMine?: () => void
  /** The top bar's search was pressed: the search screen opens. */
  onSearch: () => void
}

/**
 * The first screen of Danh bạ on a phone (DESIGN.md "Danh bạ", tier 1): the title and one 64 high tile per
 * category, and the shortcut to the caller's department. It lists no one: people, tree, roles and groups
 * come once a category is opened. The top bar's search (☰ pill) is pointed at a search control here, which
 * opens the search screen.
 */
export function ContactsCategoryHome({ categories, onOpen, myDepartment, onOpenMine, onSearch }: ContactsCategoryHomeProps) {
  return (
    <div className="flex flex-col flex-1 min-h-0 overflow-y-auto bg-base">
      <div className="px-4 pt-3.5 pb-1.5">
        <h1 className="m-0 font-display text-[1.625rem] leading-tight font-bold tracking-tight text-ink">Danh bạ</h1>
      </div>
      <nav aria-label="Danh mục" className="grid content-start gap-0.5 px-1.5 pb-6">
        {categories.map((def) => (
          <CategoryItem key={def.id} def={def} selected={false} tall onOpen={() => onOpen(def.id)} />
        ))}
        {myDepartment && onOpenMine && <MyDepartmentShortcut dept={myDepartment} tall onOpen={onOpenMine} />}
      </nav>
      {/* Not drawn: the pill of the top bar finds the screen's search by this mark and presses it. */}
      <Pressable hidden aria-hidden="true" tabIndex={-1} data-module-search onClick={onSearch} />
    </div>
  )
}
