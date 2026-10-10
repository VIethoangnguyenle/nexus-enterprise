import { memo, useRef, useState, type KeyboardEvent } from 'react'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import {
  Download, FolderInput, FolderOpen, MoreHorizontal, Pencil, Share2, Trash2,
} from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import type { Arrival } from '../../hooks/useArrivals'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'
import { formatBytes, formatDateTime, formatListTime } from '../../lib/format'
import { useMotionPresets, staggerDelay, withDelay } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import type { PeopleDirectory } from '../../lib/people'
import { IconButton, MenuItem, MenuSeparator, PersonChip, Popover, Pressable } from '../primitives'
import { kindOf, ownerOf } from './drive-model'

/**
 * Name · owner · modified · size · actions (DESIGN.md §6 Table, mockup §2).
 *
 * The columns are chosen by the width of the table itself, not the window: with
 * the detail panel open the table is only ~530px wide at 1440px, too narrow for
 * five columns. Below `@2xl` the owner moves under the name (with the time), and
 * the modified column goes.
 */
const COLS =
  `grid grid-cols-[minmax(0,1fr)_88px_32px] @2xl:grid-cols-[minmax(0,1fr)_176px_96px_80px_32px]
   gap-x-3 items-center px-2.5`

export interface RowActions {
  onSelect: (item: DriveItem) => void
  onDownload: (item: DriveItem) => void
  onShare: (item: DriveItem) => void
  onMove: (item: DriveItem) => void
  onRename: (item: DriveItem) => void
  onDelete: (item: DriveItem) => void
}

interface DriveTableProps {
  actions: RowActions
  /** Table name for assistive tech, e.g. "Tệp trong Tháng 10". */
  label: string
  items: DriveItem[]
  loading: boolean
  people: PeopleDirectory
  permsOf: (item: DriveItem) => ObjectPerms
  selectedId: string | null
  /** Changes per folder; rows enter in sequence when the list is new. */
  scope: string
  /** Realtime arrivals by item version, from `useArrivals`. */
  fresh: Map<string, Arrival>
  versionOf: (item: DriveItem) => string
  /** Every row has finished folding away (the last one was deleted). */
  onRowsGone?: () => void
}

/**
 * The hairline table of Tài liệu: header 12px/600, rows 44px divided by a 1px
 * line, no zebra, no outer border. A row's name is the real control (a link
 * into a folder, a button that selects a file) stretched over the whole row;
 * the ⋯ menu sits above it. ↑/↓ move between rows.
 */
export function DriveTable(props: DriveTableProps) {
  const { items, loading, scope } = props
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)

  // Rows already on screen when the folder first shows enter in sequence;
  // anything that arrives later (realtime, an upload) enters on its own.
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && items.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(items.map((i, n) => [i.id, n])) }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const rows = Array.from(bodyRef.current?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
    const at = rows.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    const next = rows[e.key === 'ArrowDown' ? Math.min(at + 1, rows.length - 1) : Math.max(at - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  return (
    <div role="table" aria-label={props.label} aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Tên</span>
        <span role="columnheader" className="hidden @2xl:block">Chủ sở hữu</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Sửa lúc</span>
        <span role="columnheader" className="text-right">Dung lượng</span>
        <span role="columnheader" className="sr-only">Thao tác</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3, 4].map((i) => (
            <div key={i} role="row" className={`${COLS} h-14 @2xl:h-11 border-b border-line`}>
              <span className="flex items-center gap-2.5">
                <span className="skeleton w-7 h-7 rounded-md" />
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${70 - i * 8}%` }} />
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-3.5 w-3/4 rounded-sm ml-auto hidden @2xl:block" />
              <span className="skeleton h-3.5 w-2/3 rounded-sm ml-auto" />
              <span />
            </div>
          ))
        ) : (
          // Keyed by folder: leaving one folder swaps the rows at once; only
          // rows that appear or go while the folder is open animate.
          <AnimatePresence key={scope} onExitComplete={props.onRowsGone}>
            {items.map((item) => {
              const index = firstBatch.current?.ids.get(item.id)
              const first = index !== undefined
              const delay = first ? staggerDelay(index, m.reduced) : 0
              const arrival = props.fresh.get(props.versionOf(item))
              return (
                <motion.div
                  key={item.id}
                  // The first batch fades in row by row; a row that appears
                  // later unfolds, and any row folds away when it goes.
                  {...withDelay(first ? m.route : m.row, delay)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  // Height animates in and out; the row itself paints its own divider.
                  className="overflow-hidden"
                >
                  <DriveRow
                    item={item}
                    perms={props.permsOf(item)}
                    people={props.people}
                    selected={props.selectedId === item.id}
                    arrival={arrival}
                    actions={props.actions}
                  />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

interface DriveRowProps {
  item: DriveItem
  perms: ObjectPerms
  people: PeopleDirectory
  selected: boolean
  arrival?: Arrival
  actions: RowActions
}

const DriveRow = memo(function DriveRow({ item, perms, people, selected, arrival, actions }: DriveRowProps) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const Icon = kind.icon
  const owner = ownerOf(item, people)
  const byOther = arrival?.source === 'other'
  const menuRef = useRef<HTMLButtonElement>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  // The whole row is the target of its name control (see ::after), and keeps a
  // ring inside the row when that control has keyboard focus.
  const control = `min-w-0 truncate font-medium text-ink text-left no-underline cursor-pointer outline-none
    after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
    focus-visible:after:-outline-offset-2`

  return (
    <div
      role="row"
      style={byOther ? personStyle(owner.hueKey) : undefined}
      onContextMenu={(e) => {
        e.preventDefault()
        setMenuOpen(true)
      }}
      className={`${COLS} row-lazy relative min-h-14 @2xl:min-h-11 border-b border-line transition-colors duration-quick
        ${selected ? 'bg-accent-wash' : 'hover:bg-hover'} ${byOther ? 'rt-wash' : ''}`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0 row-start-1">
        <span className={`grid place-items-center w-7 h-7 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
          <Icon size={16} strokeWidth={1.75} />
        </span>
        {folder ? (
          <Link
            data-row
            to="/drive"
            search={(prev: DriveSearch) => folderSearch(prev, item.id)}
            className={control}
          >
            {item.name}
          </Link>
        ) : (
          <Pressable
            data-row
            aria-current={selected || undefined}
            onClick={() => actions.onSelect(item)}
            className={control}
          >
            {item.name}
          </Pressable>
        )}
        {byOther && <span className="rt-tag shrink-0 text-xs font-semibold">vừa cập nhật</span>}
      </span>
      {/* Beside the time on wide tables; under the name, with the time, on narrow ones. */}
      <span
        role="cell"
        className="flex items-center gap-1.5 min-w-0 text-xs @2xl:text-sm row-start-2 @2xl:row-start-1"
      >
        <PersonChip name={owner.name} hueKey={owner.hueKey} avatarUrl={owner.avatarUrl} />
        <span className="shrink-0 text-ink-muted tnum @2xl:hidden" title={formatDateTime(item.updated_at)}>
          · {formatListTime(item.updated_at)}
        </span>
      </span>
      <span
        role="cell"
        className="hidden @2xl:block row-start-1 text-right text-ink-muted tnum"
        title={formatDateTime(item.updated_at)}
      >
        {formatListTime(item.updated_at)}
      </span>
      <span
        role="cell"
        className="text-right text-ink-muted tnum row-start-1 row-span-2 @2xl:row-span-1 col-start-2 @2xl:col-auto"
      >
        {folder ? '' : formatBytes(item.size_bytes)}
      </span>
      <span role="cell" className="relative z-dropdown row-start-1 row-span-2 @2xl:row-span-1 col-start-3 @2xl:col-auto">
        <IconButton
          ref={menuRef}
          size="md"
          aria-label={`Thao tác với ${item.name}`}
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((o) => !o)}
        >
          <MoreHorizontal size={18} strokeWidth={1.75} />
        </IconButton>
        <Popover
          open={menuOpen}
          onClose={() => setMenuOpen(false)}
          anchorRef={menuRef}
          placement="bottom-end"
          role="menu"
          label={`Thao tác với ${item.name}`}
        >
          <RowMenu item={item} perms={perms} actions={actions} />
        </Popover>
      </span>
    </div>
  )
})

function RowMenu({ item, perms, actions }: { item: DriveItem; perms: ObjectPerms; actions: RowActions }) {
  const folder = item.item_type === 'folder'
  // Moving, renaming and deleting are all a write on the item's object
  // attribute (trash, restore and remove included): there is no separate
  // delete right to ask about.
  return (
    <>
      {folder ? (
        <MenuItem icon={<FolderOpen size={16} strokeWidth={1.75} />} onClick={() => actions.onSelect(item)}>
          Chi tiết
        </MenuItem>
      ) : (
        <MenuItem icon={<Download size={16} strokeWidth={1.75} />} onClick={() => actions.onDownload(item)}>
          Tải xuống
        </MenuItem>
      )}
      {perms.share && (
        <MenuItem icon={<Share2 size={16} strokeWidth={1.75} />} onClick={() => actions.onShare(item)}>
          Chia sẻ
        </MenuItem>
      )}
      {perms.write && (
        <MenuItem icon={<FolderInput size={16} strokeWidth={1.75} />} onClick={() => actions.onMove(item)}>
          Di chuyển
        </MenuItem>
      )}
      {perms.write && (
        <MenuItem icon={<Pencil size={16} strokeWidth={1.75} />} onClick={() => actions.onRename(item)}>
          Đổi tên
        </MenuItem>
      )}
      {perms.write && (
        <>
          <MenuSeparator />
          <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={() => actions.onDelete(item)}>
            Xoá
          </MenuItem>
        </>
      )}
    </>
  )
}
