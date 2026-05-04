import { memo, useState, useCallback, useRef, useEffect } from 'react'
import { Download, Edit3, Share2, Trash2, MoreVertical, FolderInput, FolderOpen, Folder, Star } from 'lucide-react'
import { IconButton } from '../primitives'
import type { DriveItem } from '../../api/drive'
import type { ObjectPerms } from '../../stores/permission.store'
import { getFileIcon } from '../../lib/fileIcons'

interface DriveFileRowProps {
  item: DriveItem
  perms: ObjectPerms
  isOwner: boolean
  isSelected: boolean
  onSelect: (id: string) => void
  onNavigate: (item: DriveItem) => void
  onDownload: (item: DriveItem) => void
  onRename: (item: DriveItem) => void
  onShare: (item: DriveItem) => void
  onMove: (item: DriveItem) => void
  onTrash: (item: DriveItem) => void
  onDelete?: (item: DriveItem) => void
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatDate(dateVal: unknown): string {
  if (!dateVal) return '—'
  let d: Date
  if (typeof dateVal === 'object' && dateVal !== null && 'seconds' in dateVal) {
    const secs = Number((dateVal as any).seconds)
    if (isNaN(secs)) return '—'
    d = new Date(secs * 1000)
  } else if (typeof dateVal === 'string') {
    d = new Date(dateVal)
  } else {
    return '—'
  }
  if (isNaN(d.getTime())) return '—'
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60_000) return 'Just now'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

/** Memoized file row — Stitch "Nexus Drive" design.
 *  Grid: checkbox(0.5) | Name(5) | Modified(2) | Members(2) | Size(1.5) | Actions(1) */
export const DriveFileRow = memo(function DriveFileRow({
  item, perms, isOwner, isSelected, onSelect, onNavigate, onDownload, onRename, onShare, onMove, onTrash, onDelete,
}: DriveFileRowProps) {
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  const isFolder = item.item_type === 'folder'

  const handleDoubleClick = useCallback(() => {
    if (isFolder) onNavigate(item)
  }, [isFolder, item, onNavigate])

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY })
  }, [])

  useEffect(() => {
    if (!contextMenu) return
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setContextMenu(null)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [contextMenu])

  const renderIcon = () => {
    if (isFolder) {
      return (
        <div className="w-9 h-9 rounded-[10px] bg-amber-500/8 flex items-center justify-center flex-shrink-0
          group-hover:bg-amber-500/12 transition-colors duration-150">
          <Folder size={18} className="text-amber-500" strokeWidth={1.8} />
        </div>
      )
    }
    const fi = getFileIcon(item.name, item.mime_type)
    const Icon = fi.icon
    return (
      <div className="w-9 h-9 rounded-[10px] flex items-center justify-center flex-shrink-0
        group-hover:scale-[1.02] transition-all duration-150"
        style={{ backgroundColor: `${fi.color}10` }}>
        <Icon size={18} color={fi.color} strokeWidth={1.8} />
      </div>
    )
  }

  return (
    <>
      {/* Row */}
      <div
        className={`grid grid-cols-12 gap-3 px-4 items-center group
          transition-all duration-150 cursor-pointer h-[48px]
          ${isSelected
            ? 'bg-primary-container/8 border-l-[3px] border-l-primary'
            : 'hover:bg-surface-container/60 border-l-[3px] border-l-transparent'
          }
          border-b border-b-outline-variant/30`}
        onClick={() => onSelect(item.id)}
        onDoubleClick={handleDoubleClick}
        onContextMenu={handleContextMenu}
      >
        {/* Name — col-span-5 */}
        <div className="col-span-5 flex items-center gap-3 min-w-0">
          {renderIcon()}
          <div className="min-w-0 flex-1">
            <span className={`block truncate text-[13px] leading-tight
              ${isFolder ? 'font-medium text-on-surface' : 'text-on-surface'}`}>
              {item.name}
            </span>
            {/* Subtitle on mobile — show date inline */}
            <span className="block md:hidden text-[11px] text-on-surface-variant/60 mt-0.5">
              {formatDate(item.updated_at)} · {isFolder ? 'Folder' : formatSize(item.size_bytes)}
            </span>
          </div>
        </div>

        {/* Modified — col-span-2 */}
        <div className="col-span-2 text-[12.5px] text-on-surface-variant/70 hidden md:block">
          {formatDate(item.updated_at)}
        </div>

        {/* Members — col-span-2 (avatar stack) */}
        <div className="col-span-2 hidden md:flex items-center">
          <div className="flex -space-x-1.5">
            {/* Owner avatar */}
            <div className="w-7 h-7 rounded-full bg-primary-container text-on-primary-container
              flex items-center justify-center text-[10px] font-semibold
              border-2 border-surface-container-lowest ring-1 ring-white/5
              shadow-[0_1px_2px_rgba(0,0,0,0.06)]">
              {(item.owner_id || '?')[0]?.toUpperCase()}
            </div>
          </div>
        </div>

        {/* Size — col-span-2 */}
        <div className="col-span-2 text-[12.5px] text-on-surface-variant/70 hidden md:block font-mono tracking-tight">
          {isFolder ? '—' : formatSize(item.size_bytes)}
        </div>

        {/* Actions — col-span-1, three-dot menu */}
        <div className="col-span-1 hidden md:flex items-center justify-end gap-0.5">
          <IconButton
            className="opacity-0 group-hover:opacity-100 transition-all duration-150
              hover:bg-surface-container-high"
            onClick={(e) => { e.stopPropagation(); setContextMenu({ x: e.clientX, y: e.clientY }) }}
            onMouseDown={(e) => e.stopPropagation()}
            aria-label="More actions"
            size="sm"
          >
            <MoreVertical size={15} />
          </IconButton>
        </div>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <div
          ref={menuRef}
          className="fixed z-50 bg-surface-container-lowest border border-outline-variant/60 rounded-xl
            py-1 min-w-[200px] shadow-[0_8px_30px_rgba(0,0,0,0.12),0_2px_8px_rgba(0,0,0,0.06)]
            backdrop-blur-sm animate-scale-in"
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {isFolder && (
            <MenuButton onClick={() => { setContextMenu(null); onNavigate(item) }}
              icon={<FolderOpen size={15} />} label="Open" />
          )}
          {!isFolder && (
            <MenuButton onClick={() => { setContextMenu(null); onDownload(item) }}
              icon={<Download size={15} />} label="Download" />
          )}
          {perms.share && (
            <MenuButton onClick={() => { setContextMenu(null); onShare(item) }}
              icon={<Share2 size={15} />} label="Share" />
          )}
          {perms.write && (
            <MenuButton onClick={() => { setContextMenu(null); onMove(item) }}
              icon={<FolderInput size={15} />} label="Move to…" />
          )}
          {perms.write && (
            <MenuButton onClick={() => { setContextMenu(null); onRename(item) }}
              icon={<Edit3 size={15} />} label="Rename" />
          )}
          {(perms.delete || isOwner) && (
            <>
              <div className="border-t border-outline-variant/40 my-1 mx-2.5" />
              <MenuButton onClick={() => { setContextMenu(null); onTrash(item) }}
                icon={<Trash2 size={15} />} label="Delete" danger />
            </>
          )}
        </div>
      )}
    </>
  )
})

/** Context menu button */
function MenuButton({
  onClick, icon, label, danger,
}: {
  onClick: () => void
  icon: React.ReactNode
  label: string
  danger?: boolean
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left px-3 py-[7px] text-[13px] flex items-center gap-2.5
        border-none bg-transparent cursor-pointer rounded-lg mx-0
        transition-all duration-150
        ${danger
          ? 'text-error hover:bg-error-container/20'
          : 'text-on-surface hover:bg-surface-container-high/80'
        }`}
    >
      <span className={`${danger ? 'text-error/70' : 'text-on-surface-variant/70'}`}>{icon}</span>
      {label}
    </button>
  )
}
