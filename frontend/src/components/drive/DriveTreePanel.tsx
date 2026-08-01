import { useState, useCallback, memo } from 'react'
import { ChevronRight, Folder } from 'lucide-react'
import { useDriveStore } from '../../stores/drive.store'
import { useDriveFolder } from '../../hooks/useDrive'
import { NavRow } from '../primitives'

interface TreeNodeProps {
  id: string
  name: string
  wsId: string
  depth: number
}

/** Single folder tree node with lazy-loaded children. */
const TreeNode = memo(function TreeNode({ id, name, wsId, depth }: TreeNodeProps) {
  const expanded = useDriveStore((s) => s.expandedFolders.has(id))
  const currentFolderId = useDriveStore((s) => s.currentFolderId)
  const toggleFolder = useDriveStore((s) => s.toggleFolder)
  const navigateToFolder = useDriveStore((s) => s.navigateToFolder)
  const isActive = currentFolderId === id

  // Only fetch children when expanded (lazy load)
  const { data } = useDriveFolder(wsId, expanded ? id : undefined)
  const children = data?.items?.filter((i) => i.item_type === 'folder') ?? []

  const handleClick = useCallback(() => {
    navigateToFolder(id)
  }, [id, navigateToFolder])

  const handleToggle = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation()
      toggleFolder(id)
    },
    [id, toggleFolder],
  )

  return (
    <div>
      <NavRow
        kind="subItem"
        active={isActive}
        onClick={handleClick}
        style={{ paddingLeft: `${8 + depth * 16}px` }}
      >
        <span
          onClick={handleToggle}
          className={`w-4 h-4 flex items-center justify-center transition-transform duration-150
            ${expanded ? 'rotate-90' : ''}`}
        >
          <ChevronRight size={14} />
        </span>
        <Folder size={16} className={isActive ? 'text-primary' : 'text-outline'} />
        <span className="truncate">{name}</span>
      </NavRow>

      {expanded && children.length > 0 && (
        <div className="pl-7 border-l border-outline-variant ml-3 mt-1 flex flex-col gap-0.5">
          {children.map((child) => (
            <TreeNode
              key={child.id}
              id={child.id}
              name={child.name}
              wsId={wsId}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  )
})

interface DriveTreePanelProps {
  workspaceId: string
}

/** Folder tree sidebar panel for Drive. */
export function DriveTreePanel({ workspaceId }: DriveTreePanelProps) {
  const navigateToFolder = useDriveStore((s) => s.navigateToFolder)
  const currentFolderId = useDriveStore((s) => s.currentFolderId)
  const { data, isLoading } = useDriveFolder(workspaceId)
  const folders = data?.items?.filter((i) => i.item_type === 'folder') ?? []

  return (
    <div className="py-2 px-1 overflow-y-auto h-full">
      <div className="px-2 mb-2">
        <span className="text-label-caps text-on-surface-variant">
          Drive
        </span>
      </div>

      {/* Root entry — paddingLeft matches TreeNode's `8 + depth * 16` formula at depth 0, so
          "My Drive" lines up flush with the tree below it instead of NavRow's default pl-9. */}
      <NavRow
        kind="subItem"
        active={currentFolderId === null}
        onClick={() => navigateToFolder(null)}
        style={{ paddingLeft: '8px' }}
      >
        <Folder size={16} className={currentFolderId === null ? 'text-primary' : 'text-outline'} />
        <span>My Drive</span>
      </NavRow>

      {isLoading ? (
        <div className="px-3 py-2 text-xs text-on-surface-variant">Loading…</div>
      ) : (
        folders.map((folder) => (
          <TreeNode
            key={folder.id}
            id={folder.id}
            name={folder.name}
            wsId={workspaceId}
            depth={1}
          />
        ))
      )}
    </div>
  )
}
