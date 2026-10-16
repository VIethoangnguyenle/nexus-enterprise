import type { MouseEvent } from 'react'
import { Folder } from 'lucide-react'
import { Pressable } from '../primitives/Pressable'

interface FolderTileProps {
  name: string
  /** How many files it holds. */
  count?: number
  /** With an address the tile is a real link (open in a new tab, copy link). */
  href?: string
  onOpen?: () => void
}

/** A folder in the grid: a 48 px bar (44 on a phone) with its name and file count. Put inside a `FileGrid variant="folders"`. */
export function FolderTile({ name, count, href, onOpen }: FolderTileProps) {
  const className = `flex items-center gap-2.5 min-w-0 h-11 md:h-12 pl-3 pr-2.5 rounded-xl bg-raised font-semibold text-ink no-underline
    hover:bg-overlay hover:shadow-overlay outline-none focus-visible:ring-2 focus-visible:ring-accent`
  const body = (
    <>
      <Folder size={18} aria-hidden="true" className="shrink-0 text-ink-muted" />
      <span className="flex-1 min-w-0 truncate text-left">{name}</span>
      {count !== undefined && <span className="shrink-0 text-xs font-medium text-ink-muted tabular-nums">{count}</span>}
    </>
  )

  if (href) {
    const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
      // Ctrl/Cmd/Shift/middle click belong to the browser (new tab, new window).
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
      if (!onOpen) return
      e.preventDefault()
      onOpen()
    }
    return <a data-grid-item href={href} className={className} onClick={onClick}>{body}</a>
  }
  return <Pressable data-grid-item className={className} onClick={() => onOpen?.()}>{body}</Pressable>
}
