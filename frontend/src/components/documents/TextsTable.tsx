import { memo, useRef, useState, type KeyboardEvent } from 'react'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ExternalLink, FileText, MoreHorizontal, Trash2 } from 'lucide-react'
import type { TextDocument } from '../../api/documents'
import { formatDateTime, formatListTime } from '../../lib/format'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import { UNKNOWN_PERSON, type PeopleDirectory } from '../../lib/people'
import { IconButton, MenuItem, MenuSeparator, PersonChip, Popover } from '../primitives'
import { StatusPill } from '../approval/StatusPill'
import { statusPill, titleOf } from './document-model'

/**
 * Title · owner · status · modified · actions (DESIGN.md §6 Table, mockup §2).
 * Columns are chosen by the table's own width, as in the file table: narrow, the
 * owner and time sit under the title.
 */
const COLS =
  `grid grid-cols-[minmax(0,1fr)_96px_32px] @2xl:grid-cols-[minmax(0,1fr)_176px_104px_80px_32px]
   gap-x-3 items-center px-2.5`

interface TextsTableProps {
  label: string
  docs: TextDocument[]
  loading: boolean
  people: PeopleDirectory
  /** Changes with the list shown; rows enter in sequence when it is new. */
  scope: string
  onDelete: (doc: TextDocument) => void
}

export function TextsTable({ label, docs, loading, people, scope, onDelete }: TextsTableProps) {
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && docs.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(docs.map((d, n) => [d.id, n])) }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const rows = Array.from(bodyRef.current?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
    const at = rows.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    const next = rows[e.key === 'ArrowDown' ? Math.min(at + 1, rows.length - 1) : Math.max(at - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  return (
    <div role="table" aria-label={label} aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Tiêu đề</span>
        <span role="columnheader" className="hidden @2xl:block">Chủ sở hữu</span>
        <span role="columnheader">Trạng thái</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Sửa lúc</span>
        <span role="columnheader" className="sr-only">Thao tác</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3].map((i) => (
            <div key={i} role="row" className={`${COLS} h-14 @2xl:h-11 border-b border-line`}>
              <span className="flex items-center gap-2.5">
                <span className="skeleton w-7 h-7 rounded-md" />
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${70 - i * 9}%` }} />
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-5 w-16 rounded-full" />
              <span className="skeleton h-3.5 w-3/4 rounded-sm ml-auto hidden @2xl:block" />
              <span />
            </div>
          ))
        ) : (
          <AnimatePresence key={scope}>
            {docs.map((d) => {
              const index = firstBatch.current?.ids.get(d.id)
              const first = index !== undefined
              return (
                <motion.div
                  key={d.id}
                  {...withDelay(first ? m.route : m.row, first ? staggerDelay(index, m.reduced) : 0)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <TextRow doc={d} people={people} onDelete={onDelete} />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

const TextRow = memo(function TextRow({ doc, people, onDelete }: {
  doc: TextDocument
  people: PeopleDirectory
  onDelete: (doc: TextDocument) => void
}) {
  const person = people.byUserId.get(doc.owner_id)
  const owner = doc.owner_name || person?.name || UNKNOWN_PERSON
  const title = titleOf(doc)
  const menuRef = useRef<HTMLButtonElement>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <div
      role="row"
      onContextMenu={(e) => {
        e.preventDefault()
        setMenuOpen(true)
      }}
      className={`${COLS} row-lazy relative min-h-14 @2xl:min-h-11 border-b border-line hover:bg-hover
        transition-colors duration-quick`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0 row-start-1">
        <span className="grid place-items-center w-7 h-7 rounded-md shrink-0 bg-accent-wash text-accent" aria-hidden="true">
          <FileText size={16} strokeWidth={1.75} />
        </span>
        <Link
          data-row
          to="/documents/$docId"
          params={{ docId: doc.id }}
          className="min-w-0 truncate font-medium text-ink text-left no-underline cursor-pointer outline-none
            after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
            focus-visible:after:-outline-offset-2"
        >
          {title}
        </Link>
      </span>
      <span role="cell" className="flex items-center gap-1.5 min-w-0 text-xs @2xl:text-sm row-start-2 @2xl:row-start-1">
        <PersonChip name={owner} hueKey={doc.owner_id} avatarUrl={person?.avatarUrl || undefined} />
        <span className="shrink-0 text-ink-muted tnum @2xl:hidden" title={formatDateTime(doc.updated_at)}>
          · {formatListTime(doc.updated_at)}
        </span>
      </span>
      <span role="cell" className="row-start-1 row-span-2 @2xl:row-span-1 col-start-2 @2xl:col-auto">
        <StatusPill pill={statusPill(doc.status)} />
      </span>
      <span
        role="cell"
        className="hidden @2xl:block row-start-1 text-right text-ink-muted tnum"
        title={formatDateTime(doc.updated_at)}
      >
        {formatListTime(doc.updated_at)}
      </span>
      <span role="cell" className="relative z-dropdown row-start-1 row-span-2 @2xl:row-span-1 col-start-3 @2xl:col-auto">
        <IconButton
          ref={menuRef}
          size="md"
          aria-label={`Thao tác với ${title}`}
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((o) => !o)}
        >
          <MoreHorizontal size={18} strokeWidth={1.75} />
        </IconButton>
        <Popover open={menuOpen} onClose={() => setMenuOpen(false)} anchorRef={menuRef} placement="bottom-end" role="menu" label={`Thao tác với ${title}`}>
          <MenuLink docId={doc.id} />
          {doc.can_write && (
            <>
              <MenuSeparator />
              <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={() => onDelete(doc)}>
                Xoá
              </MenuItem>
            </>
          )}
        </Popover>
      </span>
    </div>
  )
})

/** "Mở" as a real link inside the menu, so it can also be opened in a new tab. */
function MenuLink({ docId }: { docId: string }) {
  return (
    <Link
      to="/documents/$docId"
      params={{ docId }}
      role="menuitem"
      tabIndex={-1}
      className="w-full flex items-center gap-2.5 h-9 px-2.5 rounded-md text-sm font-medium text-ink no-underline
        hover:bg-hover focus-visible:bg-hover focus-ring transition-colors duration-quick"
    >
      <ExternalLink size={16} strokeWidth={1.75} aria-hidden="true" />
      Mở
    </Link>
  )
}
