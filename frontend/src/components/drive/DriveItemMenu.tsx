import { useEffect, useRef } from 'react'
import { Camera, Download, FolderInput, FolderOpen, FolderPlus, Pencil, Share2, Trash2, Upload } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { formatBytes, formatListTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Popover } from '../primitives'
import { NavSheet, SheetItem, SheetSeparator } from '../spaces/NavSheet'
import { FileThumb } from '../files/FileThumb'
import type { MenuAnchor } from '../files/use-long-press'
import { itemVersion, kindOf, ownerOf } from './drive-model'
import { RowMenu, type RowActions } from './DriveTable'

/** The item a menu is open for, and the screen point it opened at. */
export interface MenuTarget {
  item: DriveItem
  anchor: MenuAnchor
}

interface DriveItemMenuProps {
  target: MenuTarget | null
  perms: ObjectPerms
  actions: RowActions
  onClose: () => void
}

/**
 * The ⋯ menu of a card or tile, opened where the pointer or the key asked for
 * it (a right click, Shift+F10, the ⋯). The same rows as the list's menu.
 * Focus goes back to whatever held it before, since the anchor is only a point.
 */
export function DriveItemMenu({ target, perms, actions, onClose }: DriveItemMenuProps) {
  const anchorRef = useRef<HTMLSpanElement>(null)
  const opener = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (target) {
      if (!opener.current && document.activeElement instanceof HTMLElement) opener.current = document.activeElement
      return
    }
    const el = opener.current
    opener.current = null
    // After the menu's own return of focus to its anchor, which is only a point.
    if (el?.isConnected) el.focus()
  }, [target])

  return (
    <>
      <span
        ref={anchorRef}
        aria-hidden="true"
        className="fixed w-0 h-0 pointer-events-none"
        style={{ left: target?.anchor.x ?? 0, top: target?.anchor.y ?? 0 }}
      />
      <Popover
        open={!!target}
        onClose={onClose}
        anchorRef={anchorRef}
        placement="bottom-start"
        role="menu"
        label={target ? `Thao tác với ${target.item.name}` : undefined}
      >
        {target && <RowMenu item={target.item} perms={perms} actions={actions} />}
      </Popover>
    </>
  )
}

interface DriveItemSheetProps {
  item: DriveItem
  perms: ObjectPerms
  people: PeopleDirectory
  actions: RowActions
  /** Opens the file in the viewer, or the folder in the list. */
  onOpen: (item: DriveItem) => void
  onClose: () => void
}

/**
 * The phone's version of the item menu (long press or ⋯): a bottom sheet with
 * a small preview, "Loại · dung lượng · người · giờ", and the actions in 52px rows.
 */
export function DriveItemSheet({ item, perms, people, actions, onOpen, onClose }: DriveItemSheetProps) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const owner = ownerOf(item, people)
  const facts = [kind.label, folder ? null : formatBytes(item.size_bytes), owner.name, formatListTime(item.updated_at)]
    .filter(Boolean)
    .join(' · ')
  // The sheet goes first, then the action: a dialog it opens must not meet a modal layer still up.
  const then = (run: () => void) => () => { onClose(); run() }

  return (
    <NavSheet label={`Thao tác với ${item.name}`} onClose={onClose}>
      <div className="flex items-center gap-3 px-3.5 pb-2">
        {folder ? (
          <span className={`grid place-items-center w-22 aspect-[4/3] rounded-lg shrink-0 ${kind.tile}`} aria-hidden="true">
            <kind.icon size={28} strokeWidth={1.75} />
          </span>
        ) : (
          <div className="w-22 shrink-0">
            <FileThumb
              fileId={item.id}
              name={item.name}
              mimeType={item.mime_type}
              version={itemVersion(item)}
              aspect="4 / 3"
              radius={8}
              iconSize={24}
            />
          </div>
        )}
        <div className="grid min-w-0">
          <b className="truncate font-semibold">{item.name}</b>
          <span className="text-xs text-ink-muted truncate">{facts}</span>
        </div>
      </div>
      <SheetItem icon={<FolderOpen size={20} strokeWidth={1.75} />} onClick={then(() => onOpen(item))}>Mở</SheetItem>
      {!folder && (
        <SheetItem icon={<Download size={20} strokeWidth={1.75} />} onClick={then(() => actions.onDownload(item))}>
          Tải xuống
        </SheetItem>
      )}
      {perms.share && (
        <SheetItem icon={<Share2 size={20} strokeWidth={1.75} />} onClick={then(() => actions.onShare(item))}>
          Chia sẻ
        </SheetItem>
      )}
      {perms.write && (
        <>
          <SheetItem icon={<FolderInput size={20} strokeWidth={1.75} />} onClick={then(() => actions.onMove(item))}>
            Chuyển tới thư mục…
          </SheetItem>
          <SheetItem icon={<Pencil size={20} strokeWidth={1.75} />} onClick={then(() => actions.onRename(item))}>
            Đổi tên
          </SheetItem>
          <SheetSeparator />
          <SheetItem tone="danger" icon={<Trash2 size={20} strokeWidth={1.75} />} onClick={then(() => actions.onDelete(item))}>
            Xoá
          </SheetItem>
        </>
      )}
    </NavSheet>
  )
}

/** The phone's floating + opens this: what can be added to the open folder. */
export function DriveAddSheet({ onPickFiles, onTakePhoto, onNewFolder, onClose }: {
  onPickFiles: () => void
  onTakePhoto: () => void
  onNewFolder: () => void
  onClose: () => void
}) {
  const then = (run: () => void) => () => { onClose(); run() }
  return (
    <NavSheet label="Thêm mới" onClose={onClose}>
      <SheetItem icon={<Upload size={20} strokeWidth={1.75} />} onClick={then(onPickFiles)}>Tải tệp lên</SheetItem>
      <SheetItem icon={<Camera size={20} strokeWidth={1.75} />} onClick={then(onTakePhoto)}>Chụp ảnh</SheetItem>
      <SheetItem icon={<FolderPlus size={20} strokeWidth={1.75} />} onClick={then(onNewFolder)}>Thư mục mới</SheetItem>
    </NavSheet>
  )
}
