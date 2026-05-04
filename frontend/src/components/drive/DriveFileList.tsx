import { useRef, useMemo, useCallback } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import type { DriveItem } from '../../api/drive'
import { DriveFileRow } from './DriveFileRow'
import { usePermissions } from '../../hooks/usePermissions'
import { useDriveStore } from '../../stores/drive.store'
import { useAuthStore } from '../../stores/auth.store'

const ROW_HEIGHT = 48

interface DriveFileListProps {
  items: DriveItem[]
  onNavigate: (item: DriveItem) => void
  onDownload: (item: DriveItem) => void
  onRename: (item: DriveItem) => void
  onShare: (item: DriveItem) => void
  onMove: (item: DriveItem) => void
  onTrash: (item: DriveItem) => void
  onDelete?: (item: DriveItem) => void
}

/** Virtualized file list — Stitch "Nexus Drive" design.
 *  Table columns: Name | Modified | Members | Size | Actions */
export function DriveFileList({
  items, onNavigate, onDownload, onRename, onShare, onMove, onTrash, onDelete,
}: DriveFileListProps) {
  const parentRef = useRef<HTMLDivElement>(null)
  const selectedItemId = useDriveStore((s) => s.selectedItemId)
  const selectItem = useDriveStore((s) => s.selectItem)
  const clearSelection = useDriveStore((s) => s.clearSelection)
  const currentUser = useAuthStore((s) => s.user)

  // Sort: folders first, then files
  const sorted = useMemo(() => {
    const folders = items.filter((i) => i.item_type === 'folder')
    const files = items.filter((i) => i.item_type === 'file')
    return [...folders, ...files]
  }, [items])

  // Batch permission check for all visible NGAC node IDs
  const ngacIds = useMemo(() => sorted.map((i) => i.ngac_node_id).filter(Boolean), [sorted])
  const { permsMap, isLoading: permsLoading } = usePermissions(ngacIds)

  const virtualizer = useVirtualizer({
    count: sorted.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  })

  const handleBackgroundClick = useCallback((e: React.MouseEvent) => {
    if (e.target === e.currentTarget) clearSelection()
  }, [clearSelection])

  const folderCount = useMemo(() => items.filter(i => i.item_type === 'folder').length, [items])
  const fileCount = useMemo(() => items.filter(i => i.item_type === 'file').length, [items])

  return (
    <div className="flex-1 flex flex-col min-h-0 px-4 md:px-6 pb-4">
      {/* Table container — Stitch: rounded card with soft elevation */}
      <div className="bg-surface-container-lowest border border-outline-variant/50 rounded-xl overflow-hidden
        flex flex-col flex-1 shadow-[0_1px_3px_rgba(0,0,0,0.04),0_1px_2px_rgba(0,0,0,0.02)]">

        {/* Table Header */}
        <div className="grid grid-cols-12 gap-3 px-4 h-10 items-center border-b border-outline-variant/50
          bg-surface-container-low/50">
          <div className="col-span-5 flex items-center gap-2">
            <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">
              Name
            </span>
            <span className="text-on-surface-variant/30 text-[10px]">↓</span>
            {/* Item count badge */}
            <span className="text-[10px] text-on-surface-variant/40 ml-1 hidden sm:inline">
              {folderCount > 0 && `${folderCount} folder${folderCount > 1 ? 's' : ''}`}
              {folderCount > 0 && fileCount > 0 && ' · '}
              {fileCount > 0 && `${fileCount} file${fileCount > 1 ? 's' : ''}`}
            </span>
          </div>
          <div className="col-span-2 hidden md:flex items-center">
            <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">
              Modified
            </span>
          </div>
          <div className="col-span-2 hidden md:flex items-center">
            <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">
              Members
            </span>
          </div>
          <div className="col-span-2 hidden md:flex items-center">
            <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">
              Size
            </span>
          </div>
          <div className="col-span-1 hidden md:flex items-center justify-end">
            {/* actions column - no header */}
          </div>
        </div>

        {/* Virtualized rows */}
        <div
          ref={parentRef}
          onClick={handleBackgroundClick}
          className="flex-1 overflow-y-auto scrollbar-thin"
        >
          <div
            style={{ height: `${virtualizer.getTotalSize()}px`, width: '100%', position: 'relative' }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const item = sorted[virtualRow.index]
              const perms = permsMap[item.ngac_node_id] ?? { read: true, write: false, delete: false, share: false }

              return (
                <div
                  key={item.id}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <DriveFileRow
                    item={item}
                    perms={perms}
                    isOwner={!!(currentUser && (item.owner_id === currentUser.id || item.owner_id === currentUser.ngac_node_id))}
                    isSelected={selectedItemId === item.id}
                    onSelect={selectItem}
                    onNavigate={onNavigate}
                    onDownload={onDownload}
                    onRename={onRename}
                    onShare={onShare}
                    onMove={onMove}
                    onTrash={onTrash}
                    onDelete={onDelete}
                  />
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}
