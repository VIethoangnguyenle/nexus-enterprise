import { useState, useCallback, useEffect } from 'react'
import { ChevronRight, Folder, FolderOpen, Check } from 'lucide-react'
import { driveApi, type DriveItem } from '../../api/drive'
import { Spinner } from '../primitives'

interface FolderTreeNodeProps {
  item: DriveItem
  level: number
  selectedId: string | null
  onSelect: (id: string) => void
}

/**
 * Recursive folder tree node with lazy-loading children.
 *
 * Stitch tokens:
 * - Default: text-on-surface, hover bg-surface-container-high
 * - Selected: bg-primary-container, text-primary, checkmark
 * - Indent: 12px per level (tree_indent)
 * - Chevron: on-surface-variant, rotates 90° on expand
 */
function FolderTreeNode({ item, level, selectedId, onSelect }: FolderTreeNodeProps) {
  const [expanded, setExpanded] = useState(false)
  const [children, setChildren] = useState<DriveItem[] | null>(null)
  const [loading, setLoading] = useState(false)

  const isSelected = selectedId === item.id

  const handleToggle = useCallback(async () => {
    if (!expanded && children === null) {
      setLoading(true)
      const data = await driveApi.listFolder(item.id)
      setChildren(data.items.filter((i) => i.item_type === 'folder'))
      setLoading(false)
    }
    setExpanded((prev) => !prev)
  }, [expanded, children, item.id])

  const handleSelect = useCallback(() => {
    onSelect(item.id)
  }, [item.id, onSelect])

  return (
    <div>
      {/* Row */}
      <div
        className={`flex items-center gap-2 px-3 py-2 cursor-pointer transition-colors rounded-lg
          ${isSelected ? 'bg-primary-container text-primary' : 'text-on-surface hover:bg-surface-container-high'}`}
        style={{ paddingLeft: `${12 + level * 20}px` }}
        onClick={handleSelect}
      >
        {/* Expand/Collapse chevron */}
        <button
          onClick={(e) => { e.stopPropagation(); handleToggle() }}
          className="flex-shrink-0 p-0.5 border-none bg-transparent cursor-pointer
            text-on-surface-variant hover:text-on-surface transition-transform"
          style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}
        >
          <ChevronRight size={14} />
        </button>

        {/* Folder icon */}
        {expanded ? (
          <FolderOpen size={18} className={isSelected ? 'text-primary' : 'text-on-surface-variant'} />
        ) : (
          <Folder size={18} className={isSelected ? 'text-primary' : 'text-on-surface-variant'} />
        )}

        {/* Name */}
        <span className="truncate text-sm font-medium flex-1">{item.name}</span>

        {/* Checkmark for selected */}
        {isSelected && <Check size={16} className="text-primary flex-shrink-0" />}
      </div>

      {/* Children */}
      {expanded && (
        <div>
          {loading && (
            <div className="flex items-center gap-2 px-3 py-2 text-sm text-on-surface-variant"
              style={{ paddingLeft: `${12 + (level + 1) * 20}px` }}
            >
              <Spinner size="sm" />
              Loading…
            </div>
          )}
          {children?.map((child) => (
            <FolderTreeNode
              key={child.id}
              item={child}
              level={level + 1}
              selectedId={selectedId}
              onSelect={onSelect}
            />
          ))}
          {!loading && children?.length === 0 && (
            <div className="px-3 py-1.5 text-xs text-on-surface-variant italic"
              style={{ paddingLeft: `${12 + (level + 1) * 20}px` }}
            >
              No subfolders
            </div>
          )}
        </div>
      )}
    </div>
  )
}

interface FolderTreeSelectProps {
  /** Workspace ID for loading root folders. */
  workspaceId: string
  /** Currently selected folder ID. */
  selectedId: string | null
  /** Called when the user selects a folder. */
  onSelect: (folderId: string) => void
}

/**
 * Stitch M3 — Folder tree selector with lazy-loading.
 *
 * Design source: Nexus Drive - Move File/Folder (99dc16ab)
 * - "My Drive" root section with expandable children
 * - Selected state: primary-container bg + checkmark
 * - Lazy loads children on expand via driveApi.listFolder
 */
export function FolderTreeSelect({ workspaceId, selectedId, onSelect }: FolderTreeSelectProps) {
  const [rootFolders, setRootFolders] = useState<DriveItem[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [rootExpanded, setRootExpanded] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    setLoading(true)
    driveApi.listRoot(workspaceId).then((data) => {
      setRootFolders(data.items.filter((i) => i.item_type === 'folder'))
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [workspaceId])

  return (
    <div className="overflow-y-auto max-h-80">
      {/* My Drive root */}
      <div>
        <div
          className="flex items-center gap-2 px-3 py-2 cursor-pointer text-on-surface hover:bg-surface-container-high rounded-lg font-semibold text-sm"
          onClick={() => setRootExpanded((p) => !p)}
        >
          <button
            className="flex-shrink-0 p-0.5 border-none bg-transparent cursor-pointer text-on-surface-variant transition-transform"
            style={{ transform: rootExpanded ? 'rotate(90deg)' : 'rotate(0deg)' }}
          >
            <ChevronRight size={14} />
          </button>
          <Folder size={18} className="text-on-surface-variant" />
          <span className="flex-1">My Drive</span>
        </div>

        {rootExpanded && (
          <div>
            {loading && (
              <div className="flex items-center gap-2 px-3 py-2 text-sm text-on-surface-variant" style={{ paddingLeft: '32px' }}>
                <Spinner size="sm" />
                Loading folders…
              </div>
            )}
            {rootFolders?.map((folder) => (
              <FolderTreeNode
                key={folder.id}
                item={folder}
                level={1}
                selectedId={selectedId}
                onSelect={onSelect}
              />
            ))}
            {!loading && rootFolders?.length === 0 && (
              <div className="px-3 py-1.5 text-xs text-on-surface-variant italic" style={{ paddingLeft: '32px' }}>
                No folders
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
