import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { FileText, UserPlus } from 'lucide-react'
import { useDriveFolder, useDriveQuota } from '../../hooks/useDrive'
import { useTextCount } from '../../hooks/useDocuments'
import { useDriveStore } from '../../stores/drive.store'
import { sharedSearch, textsSearch, type DriveSearch } from '../../lib/drive-search'
import { formatBytes } from '../../lib/format'
import { GROUPS, type TextGroup } from '../documents/document-model'
import { Heading, SearchField } from '../primitives'
import { TreeView, type TreeNode } from '../composites/TreeView'

interface DriveTreeProps {
  workspaceId: string
  /** Workspace name: the tree's single root, as in the mockup. */
  workspaceLabel: string
  currentFolderId?: string
  /** Ancestors of the open folder, root first, ending at the folder itself. */
  trail: string[]
  sharedActive: boolean
  /** The Văn bản group open, when the body is the documents rather than files. */
  textsActive?: TextGroup | null
  /** The name filter for the open folder (or for the documents), kept by the screen. */
  query: string
  onQuery: (query: string) => void
  /** Name of the search box; defaults to the files one. */
  searchLabel?: string
  /** Open a folder; no id is the root. */
  onOpen: (folderId?: string) => void
}

/** Id of the workspace row at the top of the tree; never a real folder id. */
const ROOT = 'workspace-root'

/** Subfolders of a folder (or of the root) as tree nodes. */
export function useFolderNodes(workspaceId: string, parentId?: string, enabled = true) {
  const q = useDriveFolder(workspaceId, parentId, enabled)
  const nodes = useMemo<TreeNode[] | undefined>(
    () => q.data && (q.data.items ?? []).filter((i) => i.item_type === 'folder').map((i) => ({ id: i.id, label: i.name })),
    [q.data],
  )
  return { nodes, isLoading: !q.data && !q.isError, isError: q.isError }
}

/**
 * List panel of Tài liệu (DESIGN.md §5, mockup §2): the folder tree under the
 * workspace, then "Khác" with what was shared with me, and the quota. Panel
 * state (which folders are open) is kept in the drive store.
 */
export function DriveTree({
  workspaceId, workspaceLabel, currentFolderId, trail, sharedActive, textsActive = null, query, onQuery,
  searchLabel = 'Tìm tệp hoặc thư mục', onOpen,
}: DriveTreeProps) {
  const opened = useDriveStore((s) => s.expandedFolders)
  const toggle = useDriveStore((s) => s.toggleFolder)
  const { data: quota } = useDriveQuota(workspaceId)

  // The workspace is the tree's one root, always open; its children are the
  // folders at the top of the drive.
  const roots = useMemo<TreeNode[]>(() => [{ id: ROOT, label: workspaceLabel }], [workspaceLabel])
  const expanded = useMemo(() => new Set([ROOT, ...opened]), [opened])
  const useChildren = (parentId: string) => useFolderNodes(workspaceId, parentId === ROOT ? undefined : parentId)

  const trailIds = useMemo(() => new Set([ROOT, ...trail.slice(0, -1)]), [trail])
  const selectedId = sharedActive || textsActive ? null : currentFolderId ?? ROOT
  const draftCount = useTextCount(workspaceId, 'drafts').data ?? 0
  const used = quota?.used_bytes ?? 0
  const max = quota?.max_bytes ?? 0
  const pct = max > 0 ? Math.min(100, Math.round((used / max) * 100)) : 0

  return (
    <nav aria-label="Thư mục" className="flex flex-col min-h-0 h-full bg-base">
      <div className="grid gap-3 px-4 pt-4 pb-3">
        <Heading as="h2" look="panel">Tài liệu</Heading>
        <SearchField
          label={searchLabel}
          tone="sunk"
          value={query}
          onChange={(e) => onQuery(e.target.value)}
        />
      </div>
      <div className="flex-1 min-h-0 overflow-y-auto px-2 pb-4">
        <div className="px-2.5 pb-1 text-label text-ink-muted">Văn bản</div>
        {GROUPS.map((g) => {
          const on = textsActive === g.id
          return (
            <Link
              key={g.id}
              to="/drive"
              search={(prev: DriveSearch) => textsSearch(prev, g.id === 'all' ? undefined : g.id)}
              aria-current={on ? 'page' : undefined}
              className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-sm text-ink no-underline focus-ring
                transition-colors duration-quick ${on ? 'bg-raised font-semibold' : 'hover:bg-hover'}`}
            >
              <FileText size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
              <span className="flex-1 truncate">{g.label}</span>
              {g.id === 'drafts' && draftCount > 0 && (
                <span
                  className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
                    text-on-accent text-2xs font-semibold tnum"
                  aria-label={`${draftCount} bản nháp`}
                >
                  {draftCount}
                </span>
              )}
            </Link>
          )
        })}

        <div className="px-2.5 pt-4 pb-1 text-label text-ink-muted">Thư mục</div>
        <TreeView
          label="Thư mục"
          roots={roots}
          useChildren={useChildren}
          expanded={expanded}
          onToggle={(id) => id !== ROOT && toggle(id)}
          selectedId={selectedId}
          trailIds={trailIds}
          onSelect={(id) => onOpen(id === ROOT ? undefined : id)}
          emptyLabel="Chưa có thư mục"
        />

        <div className="px-2.5 pt-4 pb-1 text-label text-ink-muted">Khác</div>
        <Link
          to="/drive"
          search={(prev: DriveSearch) => sharedSearch(prev)}
          aria-current={sharedActive ? 'page' : undefined}
          className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-sm text-ink no-underline focus-ring
            transition-colors duration-quick ${sharedActive ? 'bg-raised font-semibold' : 'hover:bg-hover'}`}
        >
          <UserPlus size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
          Được chia sẻ với tôi
        </Link>
      </div>

      {max > 0 && (
        <div className="px-4 py-3 grid gap-1.5">
          <div
            role="progressbar"
            aria-label="Dung lượng đã dùng"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={pct}
            className="h-1.5 rounded-full bg-sunk overflow-hidden"
          >
            <div
              className={`h-full origin-left rounded-full ${pct > 90 ? 'bg-danger' : pct > 70 ? 'bg-warning' : 'bg-accent'}`}
              style={{ transform: `scaleX(${pct / 100})` }}
            />
          </div>
          <span className="text-xs text-ink-muted tnum">
            {formatBytes(used)} trên {formatBytes(max)}
          </span>
        </div>
      )}
    </nav>
  )
}
