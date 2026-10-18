import { useEffect, useRef, type ReactNode } from 'react'
import { Camera, Copy, Download, FilePlus2, FolderInput, FolderPlus, Info, Link2, Pencil, Star, StarOff, Trash2, Upload, UserPlus } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../api/access'
import { formatBytes, formatListTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Popover, Pressable } from '../primitives'
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
  starred: boolean
  onStar: (item: DriveItem) => void
  onCopyLink: (item: DriveItem) => void
  /** "Tạo bản sao" next to the file. */
  onDuplicate: (item: DriveItem) => void
  /** "Thông tin và hoạt động": who has access, what changed. */
  onInfo: (item: DriveItem) => void
  onClose: () => void
}

/** One of the four shortcuts at the top of the item sheet: a 16-radius tile with its label. */
function QuickTile({ icon, label, onClick }: { icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <Pressable onClick={onClick} className="press grid justify-items-center gap-1.5 py-2.5 rounded-2xl bg-sunk text-xs font-medium text-ink">
      <span aria-hidden="true" className="text-ink-muted">{icon}</span>
      {label}
    </Pressable>
  )
}

/**
 * The phone's item menu (⋯ or long press; DESIGN.md "Tài liệu", "Menu tệp"):
 * the item's row with "Loại · dung lượng · ai sửa", four shortcuts (Chia sẻ ·
 * Sao chép liên kết · Gắn sao · Tải xuống), then rows of 52. Only what the
 * person may do on the item's folder is offered.
 */
export function DriveItemSheet({ item, perms, people, actions, starred, onStar, onCopyLink, onDuplicate, onInfo, onClose }: DriveItemSheetProps) {
  const folder = item.item_type === 'folder'
  const kind = kindOf(item)
  const owner = ownerOf(item, people)
  const facts = [kind.label, folder ? null : formatBytes(item.size_bytes), owner.name, formatListTime(item.updated_at)]
    .filter(Boolean)
    .join(' · ')
  // The sheet goes first, then the action: a dialog it opens must not meet a modal layer still up.
  const then = (run: () => void) => () => { onClose(); run() }
  const quick: { key: string; icon: ReactNode; label: string; run: () => void }[] = []
  if (perms.share) quick.push({ key: 'share', icon: <UserPlus size={20} strokeWidth={1.75} />, label: 'Chia sẻ', run: () => actions.onShare(item) })
  quick.push({ key: 'link', icon: <Link2 size={20} strokeWidth={1.75} />, label: 'Sao chép liên kết', run: () => onCopyLink(item) })
  quick.push({
    key: 'star', icon: starred ? <StarOff size={20} strokeWidth={1.75} /> : <Star size={20} strokeWidth={1.75} />,
    label: starred ? 'Bỏ sao' : 'Gắn sao', run: () => onStar(item),
  })
  if (!folder) quick.push({ key: 'download', icon: <Download size={20} strokeWidth={1.75} />, label: 'Tải xuống', run: () => actions.onDownload(item) })

  return (
    <NavSheet label={`Thao tác với ${item.name}`} onClose={onClose}>
      <div className="flex items-center gap-3 px-3.5 pb-2">
        {folder ? (
          <span className={`grid place-items-center size-10 rounded-surface shrink-0 bg-sunk text-ink-muted`} aria-hidden="true">
            <kind.icon size={20} strokeWidth={1.75} />
          </span>
        ) : (
          <div className="w-14 shrink-0">
            <FileThumb
              fileId={item.id}
              name={item.name}
              mimeType={item.mime_type}
              version={itemVersion(item)}
              aspect="4 / 3"
              radius={8}
              iconSize={20}
            />
          </div>
        )}
        <div className="grid min-w-0">
          <b className="truncate font-semibold">{item.name}</b>
          <span className="text-xs text-ink-muted truncate">{facts}</span>
        </div>
      </div>
      <div role="group" aria-label="Lối tắt" className="grid gap-2 px-3.5 pb-2" style={{ gridTemplateColumns: `repeat(${quick.length}, minmax(0, 1fr))` }}>
        {quick.map((q) => <QuickTile key={q.key} icon={q.icon} label={q.label} onClick={then(q.run)} />)}
      </div>
      {perms.write && (
        <SheetItem icon={<FolderInput size={20} strokeWidth={1.75} />} onClick={then(() => actions.onMove(item))}>
          Chuyển tới thư mục…
        </SheetItem>
      )}
      {perms.write && !folder && (
        <SheetItem icon={<Copy size={20} strokeWidth={1.75} />} onClick={then(() => onDuplicate(item))}>
          Tạo bản sao
        </SheetItem>
      )}
      <SheetItem icon={<Info size={20} strokeWidth={1.75} />} onClick={then(() => onInfo(item))}>
        Thông tin và hoạt động
      </SheetItem>
      {perms.write && (
        <>
          <SheetItem icon={<Pencil size={20} strokeWidth={1.75} />} onClick={then(() => actions.onRename(item))}>
            Đổi tên
          </SheetItem>
          <SheetSeparator />
          <SheetItem tone="danger" icon={<Trash2 size={20} strokeWidth={1.75} />} onClick={then(() => actions.onDelete(item))}>
            Chuyển vào thùng rác
          </SheetItem>
        </>
      )}
    </NavSheet>
  )
}

/** The + of a phone or tablet opens this (DESIGN.md "Nút +"): what can be added to the open folder. */
export function DriveAddSheet({ folderLabel, onPickFiles, onTakePhoto, onNewFolder, onNewText, onClose }: {
  folderLabel: string
  onPickFiles: () => void
  onTakePhoto: () => void
  onNewFolder: () => void
  onNewText: () => void
  onClose: () => void
}) {
  const then = (run: () => void) => () => { onClose(); run() }
  const tiles = [
    { label: 'Tải tệp lên', icon: <Upload size={22} strokeWidth={1.75} />, tone: 'bg-sunk text-ink', run: onPickFiles },
    { label: 'Chụp ảnh', icon: <Camera size={22} strokeWidth={1.75} />, tone: 'bg-sunk text-ink', run: onTakePhoto },
    { label: 'Thư mục mới', icon: <FolderPlus size={22} strokeWidth={1.75} />, tone: 'bg-sunk text-ink', run: onNewFolder },
    { label: 'Văn bản mới', icon: <FilePlus2 size={22} strokeWidth={1.75} />, tone: 'bg-info-wash text-info', run: onNewText },
  ]
  const title = `Thêm vào “${folderLabel}”`
  return (
    <NavSheet label={title} onClose={onClose}>
      <h3 className="px-3.5 pb-3 font-semibold">{title}</h3>
      <div className="grid grid-cols-4 gap-2 px-3.5 pb-3">
        {tiles.map((t) => (
          <Pressable key={t.label} onClick={then(t.run)} className="press grid justify-items-center gap-1.5 text-xs font-medium text-ink">
            <span aria-hidden="true" className={`grid place-items-center size-14 rounded-2xl ${t.tone}`}>{t.icon}</span>
            {t.label}
          </Pressable>
        ))}
      </div>
    </NavSheet>
  )
}
