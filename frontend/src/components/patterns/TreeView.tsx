import { useState, useCallback } from 'react'
import { ChevronRight, Folder, FolderOpen } from 'lucide-react'

export interface TreeNode {
  id: string
  name: string
  children?: TreeNode[]
}

interface TreeViewProps {
  /** Flat list of nodes with parent_id — will be built into tree internally. */
  nodes: TreeNode[]
  /** Currently selected node ID. */
  activeId?: string
  /** Called when a node is clicked. */
  onSelect: (id: string) => void
}

interface TreeItemProps {
  node: TreeNode
  depth: number
  activeId?: string
  expandedIds: Set<string>
  onToggle: (id: string) => void
  onSelect: (id: string) => void
}

/** Recursive folder tree with expand/collapse, active highlight. */
export function TreeView({ nodes, activeId, onSelect }: TreeViewProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())

  const handleToggle = useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  return (
    <div className="flex flex-col gap-0.5 py-2">
      {nodes.map((node) => (
        <TreeItem
          key={node.id}
          node={node}
          depth={0}
          activeId={activeId}
          expandedIds={expandedIds}
          onToggle={handleToggle}
          onSelect={onSelect}
        />
      ))}
    </div>
  )
}

/** Single tree item — renders children recursively when expanded. */
function TreeItem({ node, depth, activeId, expandedIds, onToggle, onSelect }: TreeItemProps) {
  const isActive = activeId === node.id
  const hasChildren = node.children && node.children.length > 0
  const isExpanded = expandedIds.has(node.id)
  const FolderIcon = isExpanded ? FolderOpen : Folder

  return (
    <>
      <button
        onClick={() => {
          onSelect(node.id)
          if (hasChildren) onToggle(node.id)
        }}
        className={`flex items-center gap-2 w-full text-left px-3 py-2 rounded-lg
          border-none cursor-pointer transition-all text-small
          ${isActive
            ? 'bg-primary/10 text-primary font-semibold'
            : 'bg-transparent text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
          }`}
        style={{ paddingLeft: `${12 + depth * 20}px` }}
      >
        {/* Expand chevron */}
        <span className={`flex-shrink-0 transition-transform duration-150 ${isExpanded ? 'rotate-90' : ''}`}>
          {hasChildren ? (
            <ChevronRight size={14} className="text-outline" />
          ) : (
            <span className="w-3.5" />
          )}
        </span>

        {/* Folder icon */}
        <FolderIcon size={16} className={isActive ? 'text-primary' : 'text-on-surface-variant'} />

        {/* Label */}
        <span className="truncate flex-1">{node.name}</span>
      </button>

      {/* Children */}
      {hasChildren && isExpanded && (
        <div className="animate-fade-in">
          {node.children!.map((child) => (
            <TreeItem
              key={child.id}
              node={child}
              depth={depth + 1}
              activeId={activeId}
              expandedIds={expandedIds}
              onToggle={onToggle}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </>
  )
}

/** Build a tree structure from a flat list with parent_id references. */
export function buildTree(items: { id: string; name: string; parent_id?: string }[]): TreeNode[] {
  const map = new Map<string, TreeNode>()
  const roots: TreeNode[] = []

  for (const item of items) {
    map.set(item.id, { id: item.id, name: item.name, children: [] })
  }

  for (const item of items) {
    const node = map.get(item.id)!
    if (item.parent_id && map.has(item.parent_id)) {
      map.get(item.parent_id)!.children!.push(node)
    } else {
      roots.push(node)
    }
  }

  return roots
}
