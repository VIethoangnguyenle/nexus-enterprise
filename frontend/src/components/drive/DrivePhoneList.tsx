import { memo, useRef, type PointerEvent } from 'react'
import { Link } from '@tanstack/react-router'
import { Check, MoreHorizontal, Star } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'
import { formatListTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { IconButton, Pressable } from '../primitives'
import { useLongPress } from '../files/use-long-press'
import { SwipeRow, SwipeRowGroup, type SwipeAction } from '../patterns/SwipeRow'
import { kindOf, ownerOf } from './drive-model'

/** What a row of the phone list is told about the selection. */
export interface PhoneSelection {
  /** Null outside the selection mode. */
  ids: ReadonlySet<string> | null
  /** A long press on a row: enter the mode with that row chosen. */
  start: (item: DriveItem) => void
  toggle: (item: DriveItem) => void
  /** A finger dragged down the check column reached this row: choose it. */
  add: (id: string) => void
}

interface Props {
  /** For assistive tech, e.g. "Tệp trong Tài chính". */
  label: string
  items: DriveItem[]
  people: PeopleDirectory
  permsOf: (item: DriveItem) => ObjectPerms
  starred: ReadonlySet<string>
  selection: PhoneSelection
  onOpenFile: (item: DriveItem, el?: HTMLElement) => void
  /** The row's ⋯: its sheet. */
  onMore: (item: DriveItem) => void
  /** The buttons a swipe to the left reveals; none while choosing. */
  swipe?: (item: DriveItem, perms: ObjectPerms) => SwipeAction[]
}

/**
 * The phone's list (DESIGN.md "Tài liệu", "Danh sách là mặc định trên điện
 * thoại"): rows of 60 with a 40px kind tile, the name (a star when starred) and
 * "Loại · ai sửa · khi nào", and a ⋯ of 44. A long press enters the selection
 * mode, where a check box takes the tile's place, a tap chooses, and a finger
 * drawn down the check column chooses every row it passes.
 */
export function DrivePhoneList({ label, items, people, permsOf, starred, selection, onOpenFile, onMore, swipe }: Props) {
  const listRef = useRef<HTMLUListElement>(null)
  const dragging = useRef(false)
  const choosing = selection.ids !== null

  // A finger put down on a check box and drawn down the column chooses each row it passes. The column does not
  // scroll (touch-action: none), so the rest of the row still scrolls the list.
  const onPointerDown = (e: PointerEvent) => {
    dragging.current = choosing && !!(e.target as Element).closest('[data-check]')
  }
  const onPointerMove = (e: PointerEvent) => {
    if (!dragging.current) return
    const el = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-select-id]')
    const id = el?.dataset.selectId
    if (id && listRef.current?.contains(el)) selection.add(id)
  }
  const stop = () => { dragging.current = false }

  return (
    <SwipeRowGroup>
      <ul
        ref={listRef}
        aria-label={label}
        className="grid content-start pb-28"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={stop}
        onPointerCancel={stop}
      >
        {items.map((item) => (
          <li key={item.id} data-select-id={item.id}>
            <SwipeRow id={item.id} disabled={choosing || !swipe} end={swipe?.(item, permsOf(item))}>
              <PhoneRow
                item={item}
                people={people}
                starred={starred.has(item.id)}
                choosing={choosing}
                chosen={!!selection.ids?.has(item.id)}
                selection={selection}
                onOpenFile={onOpenFile}
                onMore={onMore}
              />
            </SwipeRow>
          </li>
        ))}
      </ul>
    </SwipeRowGroup>
  )
}

const PhoneRow = memo(function PhoneRow({ item, people, starred, choosing, chosen, selection, onOpenFile, onMore }: {
  item: DriveItem
  people: PeopleDirectory
  starred: boolean
  choosing: boolean
  chosen: boolean
  selection: PhoneSelection
  onOpenFile: (item: DriveItem, el?: HTMLElement) => void
  onMore: (item: DriveItem) => void
}) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const Icon = kind.icon
  const owner = ownerOf(item, people)
  const meta = [kind.label, owner.name, formatListTime(item.updated_at)].filter(Boolean).join(' · ')
  const press = useLongPress(() => {
    navigator.vibrate?.(10)
    if (choosing) selection.toggle(item)
    else selection.start(item)
  })

  const control = `min-w-0 flex-1 flex items-center gap-3 self-stretch text-left no-underline text-ink outline-none
    focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-inset rounded-md`
  const body = (
    <>
      {choosing ? (
        <span data-check aria-hidden="true" className="grid place-items-center size-10 shrink-0 touch-none">
          <span className={`grid place-items-center size-5.5 rounded-[0.4375rem] border-2
            ${chosen ? 'bg-accent border-accent text-on-accent' : 'border-ink-subtle bg-base'}`}>
            {chosen && <Check size={14} strokeWidth={2.5} />}
          </span>
        </span>
      ) : (
        <span aria-hidden="true" className={`grid place-items-center size-10 rounded-surface shrink-0 ${folder ? 'bg-sunk text-ink-muted' : kind.tile}`}>
          <Icon size={20} strokeWidth={1.75} />
        </span>
      )}
      <span className="grid min-w-0">
        <span className="flex items-center gap-1 min-w-0 text-md font-medium">
          <span className="truncate" data-name>{item.name}</span>
          {starred && <Star size={14} strokeWidth={1.75} className="shrink-0 fill-warning text-warning" aria-label="Đã gắn sao" />}
        </span>
        <span className="text-[0.8125rem] text-ink-muted truncate">{meta}</span>
      </span>
    </>
  )

  const swallowAfterHold = (e: { preventDefault: () => void }) => {
    if (press.takeFired()) {
      e.preventDefault()
      return true
    }
    return false
  }

  return (
    <div
      className={`relative flex items-center gap-1 h-15 pl-4 pr-1 select-none [-webkit-touch-callout:none]
        ${chosen ? 'bg-accent-wash' : 'bg-base'}`}
      {...press.handlers}
      onContextMenu={(e) => { if (press.isHoldContextMenu()) e.preventDefault() }}
    >
      {choosing ? (
        <Pressable
          data-row
          role="checkbox"
          aria-checked={chosen}
          onClick={(e) => { if (!swallowAfterHold(e)) selection.toggle(item) }}
          className={control}
        >
          {body}
        </Pressable>
      ) : folder ? (
        <Link
          data-row
          to="/drive"
          search={(prev: DriveSearch) => folderSearch(prev, item.id)}
          onClick={(e) => { swallowAfterHold(e) }}
          className={control}
        >
          {body}
        </Link>
      ) : (
        <Pressable
          data-row
          onClick={(e) => { if (!swallowAfterHold(e)) onOpenFile(item, e.currentTarget) }}
          className={control}
        >
          {body}
        </Pressable>
      )}
      {!choosing && (
        <IconButton size="lg" className="size-11 shrink-0" aria-label={`Thao tác với ${item.name}`} onClick={() => onMore(item)}>
          <MoreHorizontal size={20} strokeWidth={1.75} />
        </IconButton>
      )}
    </div>
  )
})
