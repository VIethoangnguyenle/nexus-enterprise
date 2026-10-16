import { useRef, useState } from 'react'
import { ChevronDown, FolderPlus, LayoutGrid, List, Upload } from 'lucide-react'
import type { FileViewMode } from '../../stores/file-view.store'
import { Button, IconButton, MenuItem, Popover, Pressable, SearchField } from '../primitives'
import { SORT_OPTIONS, TYPE_FILTERS, type SortKey, type TypeFilter } from './drive-model'
import type { DriveLayout } from './use-drive-layout'

interface DriveToolbarProps {
  layout: DriveLayout
  /** Name of the open folder, for the search box. */
  folderLabel: string
  query: string
  onQuery: (query: string) => void
  typeFilter: TypeFilter
  onTypeFilter: (filter: TypeFilter) => void
  sort: SortKey
  onSort: (sort: SortKey) => void
  mode: FileViewMode
  onMode: (mode: FileViewMode) => void
  /** Uploads and new folders go here; false in the shared view. */
  canAdd: boolean
  uploading: boolean
  onPickFiles: () => void
  onNewFolder: () => void
}

const CHIP = `press shrink-0 h-8 px-3 rounded-md cursor-pointer text-small focus-ring max-lg:h-10`
const chipTone = (on: boolean) => (on ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover')

/**
 * The one row above the files (DESIGN.md "Tài liệu: lưới, danh sách, panel xem
 * trước"): search in the open folder, type chips, sort, grid/list and upload.
 * On a tablet the chips fold into one menu chip; on a phone they scroll
 * sideways on a row of their own and the floating button takes the upload.
 */
export function DriveToolbar(props: DriveToolbarProps) {
  const { layout, typeFilter, onTypeFilter, sort, onSort, mode, onMode, canAdd } = props
  const phone = layout === 'phone'
  const typeLabel = TYPE_FILTERS.find((t) => t.id === typeFilter)!.label
  const sortLabel = SORT_OPTIONS.find((s) => s.id === sort)!.label

  return (
    <div className="flex flex-wrap items-center gap-2 px-5 pb-3 max-sm:px-4">
      <SearchField
        label="Tìm tệp hoặc thư mục"
        placeholder={`Tìm trong ${props.folderLabel}`}
        moduleSearch
        tone="sunk"
        value={props.query}
        onChange={(e) => props.onQuery(e.target.value)}
        className={`rounded-full max-lg:h-11 ${layout === 'desktop' ? 'w-60 shrink-0' : 'flex-1 min-w-0'}`}
      />

      {layout === 'tablet' ? (
        <MenuChip
          label={typeLabel}
          name="Loại tệp"
          options={TYPE_FILTERS}
          value={typeFilter}
          onPick={onTypeFilter}
        />
      ) : (
        <div
          role="radiogroup"
          aria-label="Loại tệp"
          className={`flex gap-1.5 ${phone ? 'order-last w-full flex-nowrap overflow-x-auto pb-0.5' : ''}`}
        >
          {TYPE_FILTERS.map((t) => (
            <Pressable
              key={t.id}
              role="radio"
              aria-checked={typeFilter === t.id}
              onClick={() => onTypeFilter(t.id)}
              className={`${CHIP} ${chipTone(typeFilter === t.id)}`}
            >
              {t.label}
            </Pressable>
          ))}
        </div>
      )}

      <div className={`flex items-center gap-2 ${phone ? '' : 'ml-auto'}`}>
        {!phone && (
          <MenuChip
            label={sortLabel}
            name="Sắp xếp"
            ghost
            options={SORT_OPTIONS}
            value={sort}
            onPick={onSort}
          />
        )}
        <div role="group" aria-label="Kiểu hiển thị" className="flex items-center gap-0.5 p-0.5 rounded-md bg-sunk">
          <IconButton
            size="md"
            active={mode === 'grid'}
            aria-pressed={mode === 'grid'}
            aria-label="Lưới"
            title="Lưới"
            className="max-lg:w-10 max-lg:h-10"
            onClick={() => onMode('grid')}
          >
            <LayoutGrid size={16} strokeWidth={1.75} />
          </IconButton>
          <IconButton
            size="md"
            active={mode === 'list'}
            aria-pressed={mode === 'list'}
            aria-label="Danh sách"
            title="Danh sách"
            className="max-lg:w-10 max-lg:h-10"
            onClick={() => onMode('list')}
          >
            <List size={16} strokeWidth={1.75} />
          </IconButton>
        </div>
        {canAdd && !phone && <UploadMenu {...props} />}
      </div>
    </div>
  )
}

/** The primary button; its menu holds what can be added to the open folder. */
function UploadMenu({ uploading, onPickFiles, onNewFolder }: Pick<DriveToolbarProps, 'uploading' | 'onPickFiles' | 'onNewFolder'>) {
  const ref = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button
        ref={ref}
        size="sm"
        loading={uploading}
        aria-haspopup="menu"
        aria-expanded={open}
        className="max-lg:h-11"
        onClick={() => setOpen((o) => !o)}
      >
        <Upload size={16} strokeWidth={1.75} aria-hidden="true" />
        Tải lên
      </Button>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={ref} placement="bottom-end" role="menu" label="Tải lên">
        <MenuItem icon={<Upload size={16} strokeWidth={1.75} />} onClick={onPickFiles}>Tải tệp lên</MenuItem>
        <MenuItem icon={<FolderPlus size={16} strokeWidth={1.75} />} onClick={onNewFolder}>Thư mục mới</MenuItem>
      </Popover>
    </>
  )
}

/** A chip or ghost button that opens a menu of radio choices. */
function MenuChip<T extends string>({ label, name, options, value, onPick, ghost }: {
  label: string
  name: string
  options: { id: T; label: string }[]
  value: T
  onPick: (value: T) => void
  ghost?: boolean
}) {
  const ref = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  return (
    <>
      {ghost ? (
        <Button
          ref={ref}
          variant="ghost"
          size="sm"
          aria-label={`${name}: ${label}`}
          aria-haspopup="menu"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
        >
          {label}
          <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
        </Button>
      ) : (
        <Pressable
          ref={ref}
          aria-label={`${name}: ${label}`}
          aria-haspopup="menu"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
          className={`${CHIP} inline-flex items-center gap-1 ${chipTone(value !== options[0]!.id)}`}
        >
          {label}
          <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
        </Pressable>
      )}
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={ref} placement="bottom-end" role="menu" label={name}>
        {options.map((o) => (
          <MenuItem
            key={o.id}
            role="menuitemradio"
            aria-checked={o.id === value}
            className={o.id === value ? 'font-semibold' : ''}
            onClick={() => onPick(o.id)}
          >
            {o.label}
          </MenuItem>
        ))}
      </Popover>
    </>
  )
}
