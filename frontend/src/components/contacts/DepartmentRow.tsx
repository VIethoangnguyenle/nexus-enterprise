import { ChevronRight, Folder } from 'lucide-react'
import { Pressable } from '../primitives'
import type { DeptNode } from './contacts-model'

/**
 * A department in the list, before the people: a folder square, the name, how many people are in it and
 * beneath it, and (only when it has some) how many departments lie below, so a press tells whether it goes
 * deeper. An administrator sees the mark on a department the others do not see.
 */
export function DepartmentRow({ node, onOpen }: { node: DeptNode; onOpen: (id: string) => void }) {
  return (
    <Pressable
      onClick={() => onOpen(node.id)}
      className="press flex items-center gap-3 w-full min-w-0 min-h-13 px-2 py-2 rounded-lg border-none bg-transparent text-left
        cursor-pointer hover:bg-hover focus-ring transition-colors duration-quick"
    >
      <span aria-hidden="true" className="grid place-items-center shrink-0 w-8 h-8 rounded-md bg-accent-wash text-accent">
        <Folder size={16} strokeWidth={1.75} />
      </span>
      <span className="grid flex-1 min-w-0 gap-px">
        <span className="flex items-center gap-2 min-w-0 font-medium text-ink">
          <span className="truncate">{node.name}</span>
          {node.hidden && (
            <span className="shrink-0 inline-flex items-center h-5 px-2 rounded-full bg-warning-wash text-warning text-2xs font-semibold">
              Ẩn khỏi danh bạ
            </span>
          )}
        </span>
        {node.subDepartmentCount > 0 && (
          <span className="truncate text-small text-ink-muted">{node.subDepartmentCount} phòng, nhóm bên dưới</span>
        )}
      </span>
      <span className="shrink-0 text-small text-ink-muted tnum">{node.peopleCount} người</span>
      <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />
    </Pressable>
  )
}
