import { useRef, useState } from 'react'
import { ArrowLeft, ChevronDown, ChevronRight, LayoutGrid, List, MoreHorizontal, Search, X } from 'lucide-react'
import type { FileViewMode } from '../../stores/file-view.store'
import { Heading, IconButton, MenuItem, Popover, Pressable } from '../primitives'
import { SORT_OPTIONS, type SortKey } from './drive-model'
import type { Crumb } from './DriveBreadcrumbs'

/** Grid or list, one icon: it shows what a tap switches to. */
export function ViewToggle({ mode, onMode }: { mode: FileViewMode; onMode: (mode: FileViewMode) => void }) {
  const toGrid = mode === 'list'
  return (
    <IconButton
      size="lg"
      className="size-11"
      aria-label={toGrid ? 'Xem dạng lưới' : 'Xem dạng danh sách'}
      title={toGrid ? 'Xem dạng lưới' : 'Xem dạng danh sách'}
      onClick={() => onMode(toGrid ? 'grid' : 'list')}
    >
      {toGrid ? <LayoutGrid size={20} strokeWidth={1.75} /> : <List size={20} strokeWidth={1.75} />}
    </IconButton>
  )
}

/** "Sửa gần nhất ⌄": the sort, as a menu of the three orders. */
export function SortMenu({ sort, onSort }: { sort: SortKey; onSort: (sort: SortKey) => void }) {
  const ref = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const label = SORT_OPTIONS.find((s) => s.id === sort)!.label
  return (
    <>
      <Pressable
        ref={ref}
        aria-label={`Sắp xếp: ${label}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="press inline-flex items-center gap-1 h-11 px-1 rounded-md text-small-ui font-medium text-ink"
      >
        {label}
        <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={ref} placement="bottom-start" role="menu" label="Sắp xếp">
        {SORT_OPTIONS.map((o) => (
          <MenuItem
            key={o.id}
            role="menuitemradio"
            aria-checked={o.id === sort}
            className={o.id === sort ? 'font-semibold' : ''}
            onClick={() => { setOpen(false); onSort(o.id) }}
          >
            {o.label}
          </MenuItem>
        ))}
      </Popover>
    </>
  )
}

/**
 * The top of a folder on a phone (DESIGN.md "Tài liệu", "Trong thư mục"):
 * ← up to the parent (never straight home) · the folder's name · Tìm (scoped
 * to the folder) · ⋯; under it the path, which scrolls sideways and jumps on a
 * tap, then one row with the sort and the view switch.
 */
export function DriveFolderPhone({ title, crumbs, onUp, onCrumb, onSearch, onMore, sort, onSort, mode, onMode }: {
  title: string | undefined
  /** From the top (the workspace) to the open folder. */
  crumbs: Crumb[]
  onUp: () => void
  /** A tap on a crumb: its folder, or the top when it has no id. */
  onCrumb: (id?: string) => void
  onSearch: () => void
  /** The folder's own sheet; absent until it is known. */
  onMore?: () => void
  sort: SortKey
  onSort: (sort: SortKey) => void
  mode: FileViewMode
  onMode: (mode: FileViewMode) => void
}) {
  return (
    <header className="grid gap-0.5 pt-1">
      <div className="flex items-center gap-1 px-2">
        <IconButton size="lg" className="size-11 shrink-0" aria-label="Lên thư mục cha" title="Lên thư mục cha" onClick={onUp}>
          <ArrowLeft size={20} strokeWidth={1.75} />
        </IconButton>
        {title ? (
          <Heading as="h1" look="panel" className="flex-1 min-w-0 truncate">{title}</Heading>
        ) : (
          <div className="flex-1 skeleton h-6 w-40 rounded-sm" aria-busy="true" />
        )}
        <IconButton size="lg" className="size-11 shrink-0" aria-label={`Tìm trong ${title ?? 'thư mục'}`} onClick={onSearch}>
          <Search size={20} strokeWidth={1.75} />
        </IconButton>
        {onMore && (
          <IconButton size="lg" className="size-11 shrink-0" aria-label="Tuỳ chọn thư mục" onClick={onMore}>
            <MoreHorizontal size={20} strokeWidth={1.75} />
          </IconButton>
        )}
      </div>
      <nav aria-label="Vị trí" className="flex items-center gap-1 px-4 overflow-x-auto whitespace-nowrap text-[0.8125rem] text-ink-muted [scrollbar-width:none]">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1
          return (
            <span key={c.id ?? 'top'} className="flex items-center gap-1 shrink-0">
              {i > 0 && <ChevronRight size={14} strokeWidth={1.75} aria-hidden="true" />}
              {last ? (
                <span aria-current="page" className="font-semibold text-ink">{c.label}</span>
              ) : (
                <Pressable onClick={() => onCrumb(c.id)} className="py-1.5 rounded-sm text-inherit">{c.label}</Pressable>
              )}
            </span>
          )
        })}
      </nav>
      <div className="flex items-center justify-between px-3">
        <SortMenu sort={sort} onSort={onSort} />
        <ViewToggle mode={mode} onMode={onMode} />
      </div>
    </header>
  )
}

/** The top bar while choosing (DESIGN.md "Chọn nhiều"): ✕ · "Đã chọn N" · Chọn tất cả. */
export function DriveSelectionHeader({ count, onExit, onAll }: { count: number; onExit: () => void; onAll: () => void }) {
  return (
    <header className="flex items-center gap-1 px-2 pt-1 pb-1">
      <IconButton size="lg" className="size-11 shrink-0" aria-label="Thoát chọn" title="Thoát chọn" onClick={onExit}>
        <X size={20} strokeWidth={1.75} />
      </IconButton>
      <Heading as="h1" look="panel" className="flex-1 min-w-0 truncate">Đã chọn {count}</Heading>
      <Pressable onClick={onAll} className="press h-11 px-3 rounded-md text-small-ui font-semibold text-accent">
        Chọn tất cả
      </Pressable>
    </header>
  )
}
