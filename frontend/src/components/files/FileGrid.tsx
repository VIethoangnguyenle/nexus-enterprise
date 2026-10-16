import { useEffect, useRef, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react'
import type { MenuAnchor } from './use-long-press'

/**
 * `cards`: files, at least 200 wide, 12 apart. `folders`: the same columns, 10
 * apart. `media`: square pictures at least 112 wide, 4 apart. A phone gets two
 * columns of cards or folders (8 apart) and three of media.
 */
export type FileGridVariant = 'cards' | 'folders' | 'media'

const LAYOUT: Record<FileGridVariant, string> = {
  cards: 'grid-cols-2 gap-2 md:grid-cols-[repeat(auto-fill,minmax(200px,1fr))] md:gap-3',
  folders: 'grid-cols-2 gap-2 md:grid-cols-[repeat(auto-fill,minmax(200px,1fr))] md:gap-2.5',
  media: 'grid-cols-3 gap-1 md:grid-cols-[repeat(auto-fill,minmax(112px,1fr))] md:gap-1',
}

interface FileGridProps {
  /** Names the group for assistive technology ("Tệp", "Thư mục"). */
  label: string
  variant?: FileGridVariant
  /** Enter or Space on the focused item. */
  onOpen?: (index: number) => void
  /** Shift+F10, the menu key or a right click on an item. */
  onMenu?: (index: number, anchor: MenuAnchor) => void
  /** Items: each direct child that should be reachable carries `data-grid-item`. */
  children: ReactNode
}

const ITEM = '[data-grid-item]'

/** The number of columns the browser actually laid out, read from the resolved tracks. */
function columnCount(grid: HTMLElement): number {
  const tracks = window.getComputedStyle(grid).gridTemplateColumns
  const n = tracks && tracks !== 'none' ? tracks.split(/\s+/).filter(Boolean).length : 0
  return Math.max(1, n)
}

/**
 * The grid of files, folders or pictures. It is one tab stop: arrows move by
 * the real column count (which changes with the window), Home and End jump to
 * the ends, Enter and Space open, Shift+F10 and right click ask for the menu.
 * Keys that come from a control inside an item (its ⋯) are left alone.
 */
export function FileGrid({ label, variant = 'cards', onOpen, onMenu, children }: FileGridProps) {
  const ref = useRef<HTMLDivElement>(null)
  const active = useRef(0)

  const items = () => Array.from(ref.current?.querySelectorAll<HTMLElement>(ITEM) ?? [])

  // Roving tab stop: only the item last focused (the first, at the start) can be tabbed to.
  useEffect(() => {
    const all = items()
    if (active.current >= all.length) active.current = Math.max(0, all.length - 1)
    all.forEach((el, i) => { el.tabIndex = i === active.current ? 0 : -1 })
  })

  const indexOf = (target: EventTarget | null): number => {
    const el = (target as HTMLElement | null)?.closest<HTMLElement>(ITEM)
    return el ? items().indexOf(el) : -1
  }

  const focusItem = (index: number) => {
    const all = items()
    const el = all[index]
    if (!el) return
    active.current = index
    all.forEach((other, i) => { other.tabIndex = i === index ? 0 : -1 })
    el.focus()
  }

  const anchorOf = (el: HTMLElement): MenuAnchor => {
    const r = el.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const all = items()
    const index = all.findIndex((el) => el === e.target)
    if (index < 0) return // from inside an item, or from the grid itself
    const last = all.length - 1
    const cols = columnCount(ref.current!)
    let next = -1

    switch (e.key) {
      case 'ArrowRight': next = Math.min(index + 1, last); break
      case 'ArrowLeft': next = Math.max(index - 1, 0); break
      case 'ArrowDown':
        // A shorter last row is still a row below: land on its last item.
        next = index + cols <= last ? index + cols : Math.floor(index / cols) < Math.floor(last / cols) ? last : index
        break
      case 'ArrowUp': next = index - cols >= 0 ? index - cols : index; break
      case 'Home': next = 0; break
      case 'End': next = last; break
      case 'Enter':
      case ' ':
        e.preventDefault() // also keeps a link from following itself and Space from scrolling
        onOpen?.(index)
        return
      case 'ContextMenu':
        e.preventDefault()
        onMenu?.(index, anchorOf(all[index]!))
        return
      case 'F10':
        if (!e.shiftKey) return
        e.preventDefault()
        onMenu?.(index, anchorOf(all[index]!))
        return
      default:
        return
    }
    e.preventDefault()
    focusItem(next)
  }

  const onContextMenu = (e: MouseEvent<HTMLDivElement>) => {
    if (!onMenu) return
    const index = indexOf(e.target)
    if (index < 0) return
    e.preventDefault()
    onMenu(index, { x: e.clientX, y: e.clientY })
  }

  return (
    <div
      ref={ref}
      data-testid="grid"
      role="group"
      aria-label={label}
      className={`grid min-w-0 ${LAYOUT[variant]}`}
      onKeyDown={onKeyDown}
      onContextMenu={onContextMenu}
      onFocus={(e) => {
        const index = indexOf(e.target)
        const all = items()
        if (index < 0 || all[index] !== e.target) return
        active.current = index
        all.forEach((el, i) => { el.tabIndex = i === index ? 0 : -1 })
      }}
    >
      {children}
    </div>
  )
}
