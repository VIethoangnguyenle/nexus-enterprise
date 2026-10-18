import type { ReactNode } from 'react'
import { Download, FolderInput, Star, StarOff, Trash2, UserPlus } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { Pressable, toast } from '../primitives'

export interface SelectionActions {
  onShare: (item: DriveItem) => void
  onMove: (items: DriveItem[]) => void
  onDownload: (items: DriveItem[]) => void
  onStar: (items: DriveItem[], star: boolean) => void
  onDelete: (items: DriveItem[]) => void
}

interface Action {
  key: string
  label: string
  icon: ReactNode
  /** Why it cannot be done for this selection; absent when it can. */
  refused?: string
  run: () => void
}

/**
 * What the dark bar offers for a selection: only what every chosen item
 * allows, by the policy's answer on each item's folder (the batch access
 * check). An action that is not allowed stays in its place, dimmed, and a tap
 * says why.
 */
export function selectionActions(items: DriveItem[], permsOf: (i: DriveItem) => ObjectPerms, starred: ReadonlySet<string>, on: SelectionActions): Action[] {
  const none = items.length === 0 ? 'Chọn ít nhất một mục' : undefined
  const every = (ok: (i: DriveItem) => boolean) => items.length > 0 && items.every(ok)
  const allStarred = every((i) => starred.has(i.id))
  return [
    {
      key: 'share', label: 'Chia sẻ', icon: <UserPlus size={20} strokeWidth={1.75} />,
      refused: none ?? (items.length > 1 ? 'Chọn một mục để chia sẻ' : !permsOf(items[0]!).share ? 'Bạn không có quyền chia sẻ mục này' : undefined),
      run: () => on.onShare(items[0]!),
    },
    {
      key: 'move', label: 'Chuyển', icon: <FolderInput size={20} strokeWidth={1.75} />,
      refused: none ?? (every((i) => permsOf(i).write) ? undefined : 'Bạn không có quyền chuyển mọi mục đã chọn'),
      run: () => on.onMove(items),
    },
    {
      key: 'download', label: 'Tải xuống', icon: <Download size={20} strokeWidth={1.75} />,
      refused: none ?? (every((i) => i.item_type === 'file') ? undefined : 'Chỉ tải xuống được tệp, không tải được thư mục'),
      run: () => on.onDownload(items),
    },
    {
      key: 'star', label: allStarred ? 'Bỏ sao' : 'Gắn sao',
      icon: allStarred ? <StarOff size={20} strokeWidth={1.75} /> : <Star size={20} strokeWidth={1.75} />,
      refused: none,
      run: () => on.onStar(items, !allStarred),
    },
    {
      key: 'delete', label: 'Xoá', icon: <Trash2 size={20} strokeWidth={1.75} />,
      refused: none ?? (every((i) => permsOf(i).write) ? undefined : 'Bạn không có quyền xoá mọi mục đã chọn'),
      run: () => on.onDelete(items),
    },
  ]
}

/**
 * The selection's action bar (DESIGN.md "Tài liệu", "Chọn nhiều"): ink, 64
 * high, radius 22, in place of the bottom bar, which it covers.
 */
export function DriveSelectionBar({ items, permsOf, starred, actions }: {
  items: DriveItem[]
  permsOf: (i: DriveItem) => ObjectPerms
  starred: ReadonlySet<string>
  actions: SelectionActions
}) {
  return (
    <nav
      aria-label={`Thao tác với ${items.length} mục đã chọn`}
      className="fixed inset-x-3.5 bottom-[calc(1.375rem+env(safe-area-inset-bottom,0px))] z-backdrop lg:hidden
        grid grid-cols-5 h-16 px-1 rounded-[1.375rem] bg-ink text-on-accent shadow-overlay"
    >
      {selectionActions(items, permsOf, starred, actions).map((a) => (
        <Pressable
          key={a.key}
          aria-disabled={a.refused ? true : undefined}
          onClick={() => (a.refused ? toast(a.refused) : a.run())}
          className={`grid place-items-center content-center gap-0.5 rounded-2xl text-xs font-medium cursor-pointer
            focus-ring ${a.refused ? 'opacity-40' : 'active:bg-ink-muted'}`}
        >
          {a.icon}
          {a.label}
        </Pressable>
      ))}
    </nav>
  )
}
