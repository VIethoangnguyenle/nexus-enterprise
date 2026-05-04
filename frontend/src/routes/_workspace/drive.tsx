import { createFileRoute } from '@tanstack/react-router'
import { useState, useRef, useCallback, useMemo } from 'react'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useDriveFolder, useCreateFolder, useUploadFile, useTrashItem, useDeleteItem, useRenameItem, useMoveItem } from '../../hooks/useDrive'
import { driveApi, type DriveItem } from '../../api/drive'
import { useDriveStore } from '../../stores/drive.store'
import { DriveFileList } from '../../components/drive/DriveFileList'
import { DriveContextPanel } from '../../components/drive/DriveContextPanel'
import { DriveSidebar } from '../../components/drive/DriveSidebar'
import { DriveFilterPills, matchesFileTypeFilter, type FileTypeFilter } from '../../components/drive/DriveFilterPills'
import { DeleteConfirmDialog } from '../../components/drive/DeleteConfirmDialog'
import { ShareDialog } from '../../components/drive/ShareDialog'
import { ConfirmDialog } from '../../components/composites'
import { MoveItemDialog } from '../../components/drive/MoveItemDialog'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { Button, Spinner } from '../../components/primitives'
import { ResponsiveDetailPanel } from '../../components/composites/ResponsiveDetailPanel'
import { FolderPlus, Upload, FolderOpen, Plus, List, LayoutGrid, AlertTriangle, ChevronRight, Home } from 'lucide-react'

export const Route = createFileRoute('/_workspace/drive')({
  component: DriveIndex,
})

/** Drive page — Stitch "Nexus Drive" design.
 *  Layout: Sidebar | Main (Header + Breadcrumbs + Filters + Table | ContextPanel) */
function DriveIndex() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const [viewMode, setViewMode] = useState<'list' | 'grid'>('list')
  const [fileTypeFilter, setFileTypeFilter] = useState<FileTypeFilter>('all')

  const currentFolderId = useDriveStore((s) => s.currentFolderId)
  const navigateToFolder = useDriveStore((s) => s.navigateToFolder)
  const contextPanelOpen = useDriveStore((s) => s.contextPanelOpen)
  const openContextPanel = useDriveStore((s) => s.openContextPanel)

  const { data, isLoading, error, refetch } = useDriveFolder(wsId, currentFolderId ?? undefined)
  const createFolder = useCreateFolder(wsId)
  const uploadFile = useUploadFile(wsId)
  const trashItem = useTrashItem(wsId)
  const deleteItem = useDeleteItem(wsId)
  const renameItem = useRenameItem(wsId)
  const moveItem = useMoveItem(wsId)

  // Dialog states
  const [deleteTarget, setDeleteTarget] = useState<DriveItem | null>(null)
  const [moveTarget, setMoveTarget] = useState<DriveItem | null>(null)
  const [permanentDeleteTarget, setPermanentDeleteTarget] = useState<DriveItem | null>(null)
  const [shareTarget, setShareTarget] = useState<DriveItem | null>(null)

  const fileInputRef = useRef<HTMLInputElement>(null)
  const [showNewFolder, setShowNewFolder] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')
  const [folderStack, setFolderStack] = useState<{ id: string; name: string }[]>([])

  const handleNavigate = useCallback((item: DriveItem) => {
    if (item.item_type === 'folder') {
      setFolderStack((prev) => [...prev, { id: item.id, name: item.name }])
      navigateToFolder(item.id)
    }
  }, [navigateToFolder])

  const handleBreadcrumb = useCallback((index: number) => {
    if (index === -1) {
      setFolderStack([])
      navigateToFolder(null)
    } else {
      setFolderStack((prev) => prev.slice(0, index + 1))
      navigateToFolder(folderStack[index].id)
    }
  }, [folderStack, navigateToFolder])

  const handleDownload = useCallback(async (item: DriveItem) => {
    const { download_url } = await driveApi.getDownloadUrl(item.id)
    const a = document.createElement('a')
    a.href = download_url
    a.download = item.name
    a.click()
  }, [])

  const handleRename = useCallback(async (item: DriveItem) => {
    const newName = prompt('New name:', item.name)
    if (newName && newName !== item.name) {
      await renameItem.mutateAsync({ itemId: item.id, newName })
    }
  }, [renameItem])

  const handleShare = useCallback((item: DriveItem) => {
    setShareTarget(item)
  }, [])

  const handleTrash = useCallback((item: DriveItem) => {
    setDeleteTarget(item)
  }, [])

  const handleConfirmDelete = useCallback(async (item: DriveItem) => {
    await trashItem.mutateAsync(item.id)
    setDeleteTarget(null)
  }, [trashItem])

  const handlePermanentDelete = useCallback((item: DriveItem) => {
    setPermanentDeleteTarget(item)
  }, [])

  const handleConfirmPermanentDelete = useCallback(async (item: DriveItem) => {
    await deleteItem.mutateAsync(item.id)
    setPermanentDeleteTarget(null)
  }, [deleteItem])

  const handleMove = useCallback((item: DriveItem) => {
    setMoveTarget(item)
  }, [])

  const handleConfirmMove = useCallback(async (item: DriveItem, destinationFolderId: string) => {
    await moveItem.mutateAsync({ itemId: item.id, newParentId: destinationFolderId })
    setMoveTarget(null)
  }, [moveItem])

  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) return
    await createFolder.mutateAsync({ name: newFolderName.trim(), parentId: currentFolderId ?? undefined })
    setNewFolderName('')
    setShowNewFolder(false)
  }

  const handleUpload = async () => {
    const files = fileInputRef.current?.files
    if (!files?.length) return
    for (const file of Array.from(files)) {
      await uploadFile.mutateAsync({ file, parentId: currentFolderId ?? undefined })
    }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const items = data?.items || []

  // Apply client-side file type filter (folders always pass)
  const filteredItems = useMemo(() => {
    if (fileTypeFilter === 'all') return items
    return items.filter((i) => i.item_type === 'folder' || matchesFileTypeFilter(i.mime_type, fileTypeFilter))
  }, [items, fileTypeFilter])

  // Current folder label for header
  const currentFolderLabel = folderStack.length > 0
    ? folderStack[folderStack.length - 1].name
    : 'All Files'

  return (
    <div className="flex h-full">
      {/* Drive Sidebar */}
      <DriveSidebar
        workspaceId={wsId}
        onFolderSelect={(id) => {
          navigateToFolder(id)
          setFolderStack([])
        }}
        onNewFolder={() => setShowNewFolder(true)}
        onUpload={() => fileInputRef.current?.click()}
      />
      <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleUpload} />

      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-auto">
        {/* Page header */}
        <div className="px-5 md:px-7 pt-5 md:pt-6 pb-0">
          {/* Breadcrumbs */}
          <div className="flex items-center gap-1 text-[12.5px] mb-3">
            <button
              onClick={() => handleBreadcrumb(-1)}
              className="text-on-surface-variant/70 hover:text-on-surface bg-transparent border-none cursor-pointer
                transition-all duration-150 flex items-center gap-1 rounded-md px-1.5 py-0.5
                hover:bg-surface-container"
            >
              <Home size={13} strokeWidth={1.8} />
              All Files
            </button>
            {folderStack.map((folder, idx) => (
              <span key={folder.id} className="flex items-center gap-1">
                <ChevronRight size={11} className="text-on-surface-variant/30" />
                <button
                  onClick={() => handleBreadcrumb(idx)}
                  className={`bg-transparent border-none cursor-pointer transition-all duration-150
                    rounded-md px-1.5 py-0.5
                    ${idx === folderStack.length - 1
                      ? 'text-on-surface font-medium'
                      : 'text-on-surface-variant/70 hover:text-on-surface hover:bg-surface-container'
                    }`}
                >
                  {folder.name}
                </button>
              </span>
            ))}
          </div>

          {/* Title + Actions */}
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-[20px] font-semibold text-on-surface tracking-[-0.01em] leading-tight">
                {currentFolderLabel}
              </h1>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowNewFolder(true)}
                className="gap-1.5 rounded-lg h-8 text-[13px] border-outline-variant/60
                  hover:border-outline-variant hover:shadow-[0_1px_2px_rgba(0,0,0,0.04)]
                  transition-all duration-150"
              >
                <Plus size={14} strokeWidth={2} />
                <span className="hidden sm:inline">New Folder</span>
              </Button>
              <Button
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadFile.isPending}
                size="sm"
                className="gap-1.5 rounded-lg h-8 text-[13px]
                  shadow-[0_1px_3px_rgba(0,0,0,0.08)] hover:shadow-[0_2px_6px_rgba(0,0,0,0.12)]
                  transition-all duration-150"
              >
                {uploadFile.isPending ? <Spinner size="sm" /> : <Upload size={14} strokeWidth={2} />}
                <span className="hidden sm:inline">Upload</span>
              </Button>

              {/* View mode switcher */}
              <div className="hidden md:flex items-center gap-0.5 bg-surface-container/60 rounded-lg p-0.5
                border border-outline-variant/30">
                <button
                  onClick={() => setViewMode('list')}
                  className={`p-1.5 rounded-md border-none cursor-pointer transition-all duration-150
                    ${viewMode === 'list'
                      ? 'bg-surface-container-highest text-on-surface shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                      : 'text-on-surface-variant/60 hover:text-on-surface bg-transparent'
                    }`}
                  aria-label="List view"
                >
                  <List size={14} />
                </button>
                <button
                  onClick={() => setViewMode('grid')}
                  className={`p-1.5 rounded-md border-none cursor-pointer transition-all duration-150
                    ${viewMode === 'grid'
                      ? 'bg-surface-container-highest text-on-surface shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                      : 'text-on-surface-variant/60 hover:text-on-surface bg-transparent'
                    }`}
                  aria-label="Grid view"
                >
                  <LayoutGrid size={14} />
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* File type filter pills */}
        <div className="px-5 md:px-7 pb-3">
          <DriveFilterPills active={fileTypeFilter} onChange={setFileTypeFilter} />
        </div>

        {/* New folder inline form */}
        {showNewFolder && (
          <div className="mx-5 md:mx-7 mb-3 flex items-center gap-2.5 p-3 rounded-xl
            border border-outline-variant/50 bg-surface-container-low/50
            shadow-[0_1px_3px_rgba(0,0,0,0.04)]">
            <div className="w-8 h-8 rounded-lg bg-amber-500/10 flex items-center justify-center flex-shrink-0">
              <FolderPlus size={16} className="text-amber-500" />
            </div>
            <input
              autoFocus
              type="text"
              value={newFolderName}
              onChange={(e) => setNewFolderName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCreateFolder()
                if (e.key === 'Escape') { setShowNewFolder(false); setNewFolderName('') }
              }}
              placeholder="Folder name"
              className="flex-1 px-3 py-1.5 text-[13px] bg-surface-container-lowest border border-outline-variant/50
                rounded-lg text-on-surface focus:outline-none focus:border-primary/40 transition-colors"
            />
            <Button size="sm" onClick={handleCreateFolder} disabled={createFolder.isPending}
              className="rounded-lg h-8 text-[13px]">
              {createFolder.isPending ? <Spinner size="sm" /> : 'Create'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => { setShowNewFolder(false); setNewFolderName('') }}
              className="rounded-lg h-8 text-[13px]">
              Cancel
            </Button>
          </div>
        )}

        {/* Content */}
        {isLoading ? (
          <LoadingState />
        ) : error ? (
          <ErrorState title="Failed to load drive" message={error.message} onRetry={() => refetch()} />
        ) : filteredItems.length > 0 ? (
          <DriveFileList
            items={filteredItems}
            onNavigate={handleNavigate}
            onDownload={handleDownload}
            onRename={handleRename}
            onShare={handleShare}
            onMove={handleMove}
            onTrash={handleTrash}
            onDelete={handlePermanentDelete}
          />
        ) : (
          <div className="flex-1 flex flex-col min-h-0 px-4 md:px-6 pb-4">
            <div className="bg-surface-container-lowest border border-outline-variant/50 rounded-xl overflow-hidden flex flex-col flex-1
              shadow-[0_1px_3px_rgba(0,0,0,0.04),0_1px_2px_rgba(0,0,0,0.02)]">
              {/* Empty state header */}
              <div className="grid grid-cols-12 gap-3 px-4 h-10 items-center border-b border-outline-variant/50
                bg-surface-container-low/50">
                <div className="col-span-5">
                  <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">Name</span>
                </div>
                <div className="col-span-2 hidden md:block">
                  <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">Modified</span>
                </div>
                <div className="col-span-2 hidden md:block">
                  <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">Members</span>
                </div>
                <div className="col-span-2 hidden md:block">
                  <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-on-surface-variant/60">Size</span>
                </div>
              </div>
              <div className="flex-1 flex items-center justify-center py-20">
                <EmptyState
                  icon={
                    <div className="w-16 h-16 rounded-2xl bg-surface-container flex items-center justify-center mb-2">
                      <FolderOpen size={32} className="text-on-surface-variant/25" strokeWidth={1.2} />
                    </div>
                  }
                  title={currentFolderId ? 'This folder is empty' : 'Your Drive is empty'}
                  description="Upload files or create folders to get started."
                />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Context Panel — overlay on mobile, inline on desktop */}
      {contextPanelOpen && (
        <ResponsiveDetailPanel>
          <DriveContextPanel />
        </ResponsiveDetailPanel>
      )}

      {/* Delete Confirmation Dialog */}
      <DeleteConfirmDialog
        item={deleteTarget}
        isDeleting={trashItem.isPending}
        onConfirm={handleConfirmDelete}
        onClose={() => setDeleteTarget(null)}
      />

      {/* Permanent Delete Confirmation */}
      <ConfirmDialog
        open={!!permanentDeleteTarget}
        onClose={() => setPermanentDeleteTarget(null)}
        onConfirm={() => permanentDeleteTarget && handleConfirmPermanentDelete(permanentDeleteTarget)}
        title={`Permanently delete ${permanentDeleteTarget?.item_type === 'folder' ? 'folder' : 'file'}`}
        description={
          <>
            Are you sure you want to <strong>permanently delete</strong>{' '}
            <span className="font-semibold text-on-surface">{permanentDeleteTarget?.name}</span>?
          </>
        }
        warning="This action cannot be undone. The file will be permanently removed and cannot be recovered."
        icon={<AlertTriangle size={22} className="text-error" />}
        iconBg="bg-error-container"
        confirmLabel="Delete permanently"
        confirmVariant="error"
        loading={deleteItem.isPending}
      />

      {/* Share Dialog */}
      <ShareDialog
        item={shareTarget}
        workspaceId={wsId}
        onClose={() => setShareTarget(null)}
      />

      {/* Move Item Dialog */}
      <MoveItemDialog
        item={moveTarget}
        workspaceId={wsId}
        isMoving={moveItem.isPending}
        onConfirm={handleConfirmMove}
        onClose={() => setMoveTarget(null)}
      />
    </div>
  )
}
