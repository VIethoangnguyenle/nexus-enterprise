import type { MouseEvent } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { Pressable } from '../primitives/Pressable'
import { Avatar } from '../primitives/Avatar'
import { kindOf, type Owner } from '../drive/drive-model'
import { FileThumb } from './FileThumb'
import { useCoarsePointer } from './use-coarse-pointer'
import { useLongPress, type MenuAnchor } from './use-long-press'

export type { MenuAnchor } from './use-long-press'

interface FileCardProps {
  fileId: string
  name: string
  mimeType?: string
  /** Changes when the file's content does (`itemVersion`). */
  version?: string
  /** Who made or owns it, by name; never an id. */
  owner?: Owner
  /** When, already formatted for the reader ("09:02", "03/10"). */
  timeLabel?: string
  selected?: boolean
  /** A click. On a phone, callers open the file here instead of selecting it. */
  onSelect?: () => void
  /** A double click. Enter and Space go through `FileGrid`'s `onOpen` instead. */
  onOpen?: () => void
  /** The ⋯ button and a long press. Right click and Shift+F10 go through `FileGrid`'s `onMenu`. */
  onMenu: (anchor: MenuAnchor) => void
}

/** "Nguyễn Thị Thu Lan" → "Thu Lan": the way the name is said, short enough for a card. */
function shortName(name: string): string {
  const words = name.trim().split(/\s+/)
  return words.length > 2 ? words.slice(-2).join(' ') : words.join(' ')
}

/**
 * A file in the grid: its picture, what it is called and who touched it last
 * (DESIGN.md "Tài liệu: lưới, danh sách, panel xem trước"). Put inside a
 * `FileGrid`, which makes the cards one tab stop and moves between them.
 */
export function FileCard({ fileId, name, mimeType, version, owner, timeLabel, selected = false, onSelect, onOpen, onMenu }: FileCardProps) {
  const coarse = useCoarsePointer()
  const press = useLongPress(onMenu)
  const kind = kindOf({ item_type: 'file', name, mime_type: mimeType })
  const KindIcon = kind.icon

  const surface = selected
    ? 'bg-accent-wash ring-2 ring-inset ring-accent'
    : 'bg-raised hover:bg-overlay hover:shadow-overlay'
  const moreShown = coarse || selected ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'

  const openMenuFrom = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    const r = e.currentTarget.getBoundingClientRect()
    onMenu({ x: r.right, y: r.bottom })
  }

  return (
    <div
      data-grid-item
      role="button"
      aria-label={name}
      aria-pressed={selected}
      className={`group relative grid gap-2 min-w-0 rounded-xl px-1.5 pt-1.5 pb-2.5 cursor-default select-none
        outline-none focus-visible:ring-2 focus-visible:ring-accent ${surface}`}
      {...press.handlers}
      onClick={() => { if (!press.takeFired()) onSelect?.() }}
      onDoubleClick={() => onOpen?.()}
      onContextMenu={(e) => {
        // A hold already opened the menu; the browser's own follow-up must not open it twice.
        if (press.isHoldContextMenu()) { e.preventDefault(); e.stopPropagation() }
      }}
    >
      <FileThumb fileId={fileId} name={name} mimeType={mimeType} version={version} size="card" />
      <div className="flex items-center gap-2 min-w-0 px-1">
        <span className={`grid place-items-center shrink-0 w-5 h-5 rounded-sm ${kind.tile}`} title={kind.label}>
          <KindIcon size={12} aria-hidden="true" />
        </span>
        <b className="flex-1 min-w-0 truncate font-semibold">{name}</b>
      </div>
      {owner && (
        <div className="flex items-center gap-1.5 px-1 text-xs text-ink-muted whitespace-nowrap overflow-hidden">
          <Avatar name={owner.name} hueKey={owner.hueKey} src={owner.avatarUrl} size={20} />
          <span className="truncate">{shortName(owner.name)}{timeLabel ? ` · ${timeLabel}` : ''}</span>
        </div>
      )}
      <Pressable
        aria-label={`Tùy chọn cho ${name}`}
        aria-haspopup="menu"
        className={`absolute top-2.5 right-2.5 grid place-items-center w-7 h-7 rounded-lg bg-overlay text-ink shadow-overlay
          transition-opacity duration-(--duration-quick) ${moreShown}`}
        onClick={openMenuFrom}
        onDoubleClick={(e) => e.stopPropagation()}
      >
        <MoreHorizontal size={16} aria-hidden="true" />
      </Pressable>
    </div>
  )
}
