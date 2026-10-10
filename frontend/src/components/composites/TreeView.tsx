import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { ChevronRight, Folder, FolderOpen } from 'lucide-react'

export interface TreeNode {
  id: string
  label: string
  /** False for a leaf; no chevron and nothing to load. Defaults to true. */
  hasChildren?: boolean
}

interface ChildrenState {
  nodes: TreeNode[] | undefined
  isLoading: boolean
  isError: boolean
}

interface TreeViewProps {
  /** Accessible name of the tree. */
  label: string
  roots: TreeNode[]
  /**
   * Loads one node's children. Called only while that node is open, so a
   * collapsed branch costs nothing. It is a hook: pass a stable function.
   */
  useChildren: (parentId: string) => ChildrenState
  expanded: ReadonlySet<string>
  onToggle: (id: string) => void
  selectedId?: string | null
  /** Ancestors of the selected node; drawn a step stronger than their siblings. */
  trailIds?: ReadonlySet<string>
  onSelect: (id: string) => void
  /** Leading glyph; defaults to a folder that opens with its node. */
  icon?: (node: TreeNode, open: boolean) => ReactNode
  /** Text for an open node with no children. */
  emptyLabel?: string
}

const DEFAULT_EMPTY = 'Không có thư mục con'

/**
 * The one tree (DESIGN.md §6): folders in Tài liệu, the move-to picker, and
 * whatever hierarchy comes next. 16px of indent per level comes from the
 * `--depth` custom property, never from inline padding math.
 *
 * Follows the ARIA tree pattern: one tab stop, ↑/↓ between visible items, → opens
 * or steps into a node, ← closes or steps out to the parent, Enter selects.
 */
export function TreeView(props: TreeViewProps) {
  const { label, roots, selectedId } = props
  const treeRef = useRef<HTMLUListElement>(null)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [tabStop, setTabStop] = useState<string | null>(null)

  // One tab stop: the last item focused, else the selection, else the first
  // root, taking the first of those that is actually on screen. A node can
  // vanish (its parent closed, a folder deleted); without this the tree would
  // have no tab stop and keyboard users could not get back in.
  useEffect(() => {
    const shown = new Set(Array.from(treeRef.current?.querySelectorAll<HTMLElement>('[role="treeitem"]') ?? [], (el) => el.dataset.treeId))
    const next = [activeId, selectedId, roots[0]?.id].find((id) => id && shown.has(id)) ?? null
    setTabStop((cur) => (cur === next ? cur : next))
  })

  const items = () => Array.from(treeRef.current?.querySelectorAll<HTMLElement>('[role="treeitem"]') ?? [])
  const focusItem = (el: HTMLElement | undefined) => el?.focus()

  const onKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    const el = (e.target as HTMLElement).closest<HTMLElement>('[role="treeitem"]')
    if (!el) return
    const all = items()
    const at = all.indexOf(el)
    const id = el.dataset.treeId ?? ''
    const level = Number(el.getAttribute('aria-level'))
    const expandable = el.hasAttribute('aria-expanded')
    const open = el.getAttribute('aria-expanded') === 'true'

    switch (e.key) {
      case 'ArrowDown':
        focusItem(all[at + 1])
        break
      case 'ArrowUp':
        focusItem(all[at - 1])
        break
      case 'Home':
        focusItem(all[0])
        break
      case 'End':
        focusItem(all[all.length - 1])
        break
      case 'ArrowRight':
        if (expandable && !open) props.onToggle(id)
        else if (open) focusItem(all[at + 1])
        break
      case 'ArrowLeft':
        if (expandable && open) props.onToggle(id)
        else focusItem([...all.slice(0, at)].reverse().find((n) => Number(n.getAttribute('aria-level')) === level - 1))
        break
      case 'Enter':
      case ' ':
        props.onSelect(id)
        break
      default:
        return
    }
    e.preventDefault()
  }

  return (
    <ul ref={treeRef} role="tree" aria-label={label} onKeyDown={onKeyDown} className="grid gap-px m-0 p-0 list-none">
      {roots.map((n) => (
        <Branch key={n.id} node={n} level={1} tabStop={tabStop} onFocusItem={setActiveId} {...props} />
      ))}
    </ul>
  )
}

type BranchProps = Omit<TreeViewProps, 'roots' | 'label'> & {
  node: TreeNode
  level: number
  tabStop: string | null
  onFocusItem: (id: string) => void
}

function Branch(p: BranchProps) {
  const { node, level, expanded, selectedId, trailIds, tabStop } = p
  const expandable = node.hasChildren !== false
  const open = expandable && expanded.has(node.id)
  const selected = selectedId === node.id
  const inTrail = !selected && !!trailIds?.has(node.id)

  return (
    <li role="none">
      <div
        role="treeitem"
        data-tree-id={node.id}
        aria-level={level}
        aria-selected={selected}
        aria-expanded={expandable ? open : undefined}
        aria-current={selected ? 'page' : undefined}
        tabIndex={tabStop === node.id ? 0 : -1}
        onFocus={() => p.onFocusItem(node.id)}
        onClick={() => p.onSelect(node.id)}
        style={{ ['--depth' as string]: level - 1 }}
        className={`tree-indent flex items-center gap-1.5 h-9 pr-2 rounded-surface text-sm cursor-pointer
          select-none focus-ring transition-colors duration-quick
          ${selected ? 'bg-raised font-semibold text-ink' : `hover:bg-hover text-ink ${inTrail ? 'font-semibold' : ''}`}`}
      >
        {expandable ? (
          <span
            data-testid="tree-toggle"
            aria-hidden="true"
            onClick={(e) => {
              e.stopPropagation()
              p.onFocusItem(node.id)
              p.onToggle(node.id)
            }}
            className="grid place-items-center w-5 h-5 rounded-sm text-ink-muted hover:text-ink shrink-0"
          >
            <ChevronRight
              size={16}
              strokeWidth={1.75}
              className={`transition-transform duration-quick motion-reduce:transition-none ${open ? 'rotate-90' : ''}`}
            />
          </span>
        ) : (
          <span className="w-5 shrink-0" aria-hidden="true" />
        )}
        {p.icon ? p.icon(node, open) : open ? (
          <FolderOpen size={16} strokeWidth={1.75} className="text-accent shrink-0" aria-hidden="true" />
        ) : (
          <Folder size={16} strokeWidth={1.75} className="text-accent shrink-0" aria-hidden="true" />
        )}
        <span className="truncate">{node.label}</span>
      </div>
      {open && <Children {...p} />}
    </li>
  )
}

function Children(p: BranchProps) {
  const { nodes, isLoading, isError } = p.useChildren(p.node.id)
  const depth = { ['--depth' as string]: p.level }

  if (isLoading) {
    return (
      <div className="tree-indent grid gap-1.5 py-1.5 pr-2" style={depth} aria-busy="true">
        <div className="skeleton h-5 w-3/4 rounded-sm" />
        <div className="skeleton h-5 w-1/2 rounded-sm" />
      </div>
    )
  }
  if (isError) {
    return <div className="tree-indent py-1.5 pr-2 text-small text-danger" style={depth}>Không tải được thư mục con</div>
  }
  if (!nodes || nodes.length === 0) {
    return <div className="tree-indent py-1.5 pr-2 text-small text-ink-muted" style={depth}>{p.emptyLabel ?? DEFAULT_EMPTY}</div>
  }
  return (
    <ul role="group" className="grid gap-px m-0 p-0 list-none">
      {nodes.map((n) => (
        <Branch key={n.id} {...p} node={n} level={p.level + 1} />
      ))}
    </ul>
  )
}
