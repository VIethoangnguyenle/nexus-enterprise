import { useState, useCallback } from 'react'
import {
  Search, ChevronRight, ChevronDown, Folder,
  Users, Plus, Upload, Clock, Star, Trash2, HardDrive, HelpCircle,
  FolderOpen,
} from 'lucide-react'
import { Button } from '../primitives'
import { useDriveFolder, useDriveQuota } from '../../hooks/useDrive'
import { useDriveStore } from '../../stores/drive.store'
import type { DriveItem } from '../../api/drive'

interface DriveSidebarProps {
  workspaceId: string
  onFolderSelect?: (folderId: string | null) => void
  onNewFolder?: () => void
  onUpload?: () => void
}

type SidebarSection = 'all' | 'recent' | 'shared' | 'starred' | 'trash'

const NAV_ITEMS: { id: SidebarSection; label: string; icon: typeof FolderOpen }[] = [
  { id: 'all', label: 'All Files', icon: FolderOpen },
  { id: 'recent', label: 'Recent', icon: Clock },
  { id: 'shared', label: 'Shared', icon: Users },
  { id: 'starred', label: 'Starred', icon: Star },
  { id: 'trash', label: 'Trash', icon: Trash2 },
]

function formatStorageSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
  return `${(bytes / 1073741824).toFixed(1)} GB`
}

/** Drive sidebar — Stitch "Nexus Drive - File Management" design.
 *  Layout: Brand header → Search → Upload → Nav → Tree → Storage/Help */
export function DriveSidebar({ workspaceId, onFolderSelect, onNewFolder, onUpload }: DriveSidebarProps) {
  const [search, setSearch] = useState('')
  const [activeSection, setActiveSection] = useState<SidebarSection>('all')
  const currentFolderId = useDriveStore((s) => s.currentFolderId)
  const { data: quota } = useDriveQuota(workspaceId)

  const handleNavClick = useCallback((section: SidebarSection) => {
    setActiveSection(section)
    if (section === 'all') {
      onFolderSelect?.(null)
    }
  }, [onFolderSelect])

  const usedPercent = quota && quota.max_bytes > 0
    ? Math.min((quota.used_bytes / quota.max_bytes) * 100, 100)
    : 0

  return (
    <div className="hidden lg:flex w-[260px] flex-shrink-0 bg-surface-container-low border-r border-outline-variant
      flex-col h-full select-none">

      {/* ── Brand header ── */}
      <div className="px-4 pt-5 pb-4">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-primary-container flex items-center justify-center flex-shrink-0
            shadow-[0_1px_3px_rgba(0,0,0,0.08)]">
            <HardDrive size={16} className="text-on-primary-container" />
          </div>
          <div className="min-w-0">
            <h2 className="text-[14px] font-semibold text-on-surface leading-tight truncate tracking-[-0.01em]">
              Enterprise Drive
            </h2>
            <p className="text-[11px] text-on-surface-variant/70 leading-tight mt-0.5">Enterprise Tier</p>
          </div>
        </div>
      </div>

      {/* ── Search ── */}
      <div className="px-3 pb-2">
        <div className="flex items-center gap-2 px-3 h-9 bg-surface-container rounded-lg
          border border-transparent focus-within:border-primary/40 focus-within:bg-surface-container-lowest
          transition-all duration-200">
          <Search size={14} className="text-on-surface-variant/60 flex-shrink-0" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search files, folders..."
            className="bg-transparent border-none outline-none text-[13px] text-on-surface
              placeholder:text-on-surface-variant/50 w-full"
          />
        </div>
      </div>

      {/* ── Upload button — Stitch: prominent primary button ── */}
      <div className="px-3 pb-2">
        <Button
          onClick={onUpload}
          className="w-full justify-center gap-2 h-9 rounded-lg shadow-[0_1px_2px_rgba(0,0,0,0.06)]"
          size="sm"
        >
          <Upload size={15} />
          Upload File
        </Button>
      </div>

      {/* ── Navigation ── */}
      <nav className="px-2 py-1 space-y-0.5">
        {NAV_ITEMS.map(({ id, label, icon: Icon }) => {
          const isActive = activeSection === id
          return (
            <button
              key={id}
              onClick={() => handleNavClick(id)}
              className={`flex items-center gap-2.5 w-full px-3 py-[7px] text-[13px] font-medium
                rounded-lg border-none cursor-pointer transition-all duration-150
                ${isActive
                  ? 'bg-primary-container text-on-primary-container shadow-[0_1px_2px_rgba(0,0,0,0.04)]'
                  : 'text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface bg-transparent'
                }`}
            >
              <Icon size={17} className={isActive ? 'text-on-primary-container' : 'text-on-surface-variant/70'}
                strokeWidth={isActive ? 2.2 : 1.8} />
              {label}
            </button>
          )
        })}
      </nav>

      {/* ── Separator ── */}
      <div className="mx-4 my-2 border-t border-outline-variant/60" />

      {/* ── Folder Tree (All Files view) ── */}
      {activeSection === 'all' && (
        <div className="flex-1 overflow-y-auto px-2 scrollbar-thin">
          <FolderTreeSection
            workspaceId={workspaceId}
            currentFolderId={currentFolderId}
            onSelect={onFolderSelect}
            onNewFolder={onNewFolder}
          />
        </div>
      )}

      {activeSection !== 'all' && <div className="flex-1" />}

      {/* ── Footer — Storage + Help ── */}
      <div className="border-t border-outline-variant/60 px-4 py-3.5 space-y-3">
        {/* Storage Usage */}
        {quota && quota.max_bytes > 0 && (
          <div>
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-1.5">
                <HardDrive size={13} className="text-on-surface-variant/70" />
                <span className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
                  Storage
                </span>
              </div>
              <span className="text-[11px] text-on-surface-variant/70">
                {usedPercent.toFixed(0)}%
              </span>
            </div>
            <div className="h-[5px] bg-outline-variant/30 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-700 ease-out
                  ${usedPercent > 90 ? 'bg-error' : usedPercent > 70 ? 'bg-tertiary' : 'bg-primary'}`}
                style={{ width: `${usedPercent}%` }}
              />
            </div>
            <p className="text-[11px] text-on-surface-variant/70 mt-1.5">
              {formatStorageSize(quota.used_bytes)} of {formatStorageSize(quota.max_bytes)} used
            </p>
          </div>
        )}

        {/* Help */}
        <button className="flex items-center gap-2 w-full px-1 py-1 text-[12px] text-on-surface-variant/70
          hover:text-on-surface bg-transparent border-none cursor-pointer rounded-md transition-colors">
          <HelpCircle size={14} />
          Help
        </button>
      </div>
    </div>
  )
}

/* ─── Folder Tree Section ───────────── */

function FolderTreeSection({
  workspaceId,
  currentFolderId,
  onSelect,
  onNewFolder,
}: {
  workspaceId: string
  currentFolderId: string | null
  onSelect?: (id: string | null) => void
  onNewFolder?: () => void
}) {
  const { data: myData } = useDriveFolder(workspaceId)
  const items = myData?.items || []
  const folders = items.filter(i => i.item_type === 'folder')

  return (
    <div>
      {/* Section label */}
      <div className="flex items-center justify-between px-2.5 py-1.5 mb-0.5">
        <span className="text-[10px] font-bold uppercase tracking-[0.06em] text-on-surface-variant/60">
          Folders
        </span>
        {onNewFolder && (
          <button
            onClick={onNewFolder}
            className="w-5 h-5 rounded-md flex items-center justify-center bg-transparent border-none
              cursor-pointer text-on-surface-variant/50 hover:text-on-surface hover:bg-surface-container-high
              transition-all duration-150"
            title="New folder"
          >
            <Plus size={13} />
          </button>
        )}
      </div>

      {folders.length === 0 ? (
        <div className="px-3 py-4 text-[12px] text-on-surface-variant/50 text-center">
          No folders yet
        </div>
      ) : (
        folders.map(folder => (
          <TreeNode
            key={folder.id}
            item={folder}
            workspaceId={workspaceId}
            currentFolderId={currentFolderId}
            onSelect={onSelect}
            depth={0}
          />
        ))
      )}
    </div>
  )
}

/* ─── Recursive Tree Node ─────────────── */

function TreeNode({
  item,
  workspaceId,
  currentFolderId,
  onSelect,
  depth,
}: {
  item: DriveItem
  workspaceId: string
  currentFolderId: string | null
  onSelect?: (id: string | null) => void
  depth: number
}) {
  const expanded = useDriveStore((s) => s.expandedFolders.has(item.id))
  const toggleFolder = useDriveStore((s) => s.toggleFolder)
  const isActive = currentFolderId === item.id

  const { data: childData } = useDriveFolder(
    workspaceId,
    expanded ? item.id : undefined,
  )
  const children = childData?.items || []
  const childFolders = children.filter(c => c.item_type === 'folder')

  const handleToggle = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    toggleFolder(item.id)
  }, [item.id, toggleFolder])

  const handleSelect = useCallback(() => {
    onSelect?.(item.id)
  }, [item.id, onSelect])

  const indent = depth * 14

  return (
    <div>
      <div
        className={`group flex items-center gap-1 w-full rounded-lg cursor-pointer
          transition-all duration-150 py-[5px] my-[1px]
          ${isActive
            ? 'bg-primary-container/15 text-primary'
            : 'text-on-surface-variant hover:bg-surface-container-high/80 hover:text-on-surface'
          }`}
        style={{ paddingLeft: `${8 + indent}px` }}
      >
        {/* Expand toggle */}
        <button
          onClick={handleToggle}
          className="p-0.5 rounded bg-transparent border-none cursor-pointer
            text-on-surface-variant/50 hover:text-on-surface flex-shrink-0 inline-flex transition-all duration-150"
        >
          {expanded ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
        </button>

        {/* Folder icon + name */}
        <button
          onClick={handleSelect}
          className={`flex items-center gap-1.5 flex-1 min-w-0 pr-2
            bg-transparent border-none cursor-pointer text-left
            ${isActive ? 'text-primary font-medium' : 'text-inherit'}`}
        >
          <Folder size={14} className={isActive ? 'text-primary' : 'text-on-surface-variant/50'}
            strokeWidth={isActive ? 2 : 1.5} />
          <span className="truncate text-[12.5px]">{item.name}</span>
        </button>
      </div>

      {/* Children */}
      {expanded && childFolders.length > 0 && (
        <div className="ml-3.5 border-l border-outline-variant/40">
          {childFolders.map(child => (
            <TreeNode
              key={child.id}
              item={child}
              workspaceId={workspaceId}
              currentFolderId={currentFolderId}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}
