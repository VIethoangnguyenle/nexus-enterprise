import { memo, useRef, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import type { Contact } from '../../hooks/useContacts'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import { Avatar, Pressable } from '../primitives'
import { nameOf, roleLine } from './contacts-model'

/**
 * Person · department · email · status (DESIGN.md §6 Table, mockup §1). The
 * columns are chosen by the table's own width, so with the profile panel open
 * the department and email fold under the name instead of squeezing it.
 */
const COLS =
  `grid grid-cols-[minmax(0,1fr)_112px] @2xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1.3fr)_112px]
   gap-x-3 items-center px-2.5`

interface ContactsTableProps {
  label: string
  people: Contact[]
  loading: boolean
  selectedId: string | null
  isOnline: (c: Contact) => boolean
  onSelect: (c: Contact) => void
  /** Changes when the filter does; rows enter in sequence for a new list. */
  scope: string
}

export function ContactsTable({ label, people, loading, selectedId, isOnline, onSelect, scope }: ContactsTableProps) {
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && people.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(people.map((p, n) => [p.user_id, n])) }
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
        <span role="columnheader">Người</span>
        <span role="columnheader" className="hidden @2xl:block">Phòng ban</span>
        <span role="columnheader" className="hidden @2xl:block">Email</span>
        <span role="columnheader">Trạng thái</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3, 4].map((i) => (
            <div key={i} role="row" className={`${COLS} h-14 border-b border-line`}>
              <span className="flex items-center gap-2.5">
                <span className="skeleton w-8 h-8 rounded-full" />
                <span className="grid gap-1.5 flex-1">
                  <span className="skeleton h-3.5 rounded-sm" style={{ width: `${60 - i * 6}%` }} />
                  <span className="skeleton h-3 w-2/5 rounded-sm" />
                </span>
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-3.5 w-4/5 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-3.5 w-2/3 rounded-sm" />
            </div>
          ))
        ) : (
          <AnimatePresence key={scope}>
            {people.map((c) => {
              const index = firstBatch.current?.ids.get(c.user_id)
              const first = index !== undefined
              return (
                <motion.div
                  key={c.user_id}
                  {...withDelay(first ? m.route : m.row, first ? staggerDelay(index, m.reduced) : 0)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <ContactRow contact={c} selected={selectedId === c.user_id} online={isOnline(c)} onSelect={onSelect} />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

export function PresenceLabel({ online }: { online: boolean }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-sm ${online ? 'text-ink' : 'text-ink-muted'}`}>
      <span aria-hidden="true" className={`w-2 h-2 rounded-full ${online ? 'bg-success' : 'bg-line'}`} />
      {online ? 'Trực tuyến' : 'Ngoại tuyến'}
    </span>
  )
}

const ContactRow = memo(function ContactRow({ contact, selected, online, onSelect }: {
  contact: Contact
  selected: boolean
  online: boolean
  onSelect: (c: Contact) => void
}) {
  const name = nameOf(contact)
  return (
    <div
      role="row"
      className={`${COLS} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
        ${selected ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0">
        <Avatar name={name} hueKey={contact.user_id} src={contact.avatar_url || undefined} online={online} size={32} />
        <span className="grid min-w-0">
          <Pressable
            data-row
            aria-current={selected || undefined}
            onClick={() => onSelect(contact)}
            className="min-w-0 truncate font-medium text-ink cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {name}
          </Pressable>
          <span className="truncate text-xs text-ink-muted @2xl:hidden">{roleLine(contact) || contact.email}</span>
          {contact.title && <span className="hidden @2xl:block truncate text-xs text-ink-muted">{contact.title}</span>}
        </span>
      </span>
      <span role="cell" className="hidden @2xl:block truncate text-sm text-ink-muted">{contact.department}</span>
      <span role="cell" className="hidden @2xl:block truncate text-sm text-ink-muted">{contact.email}</span>
      <span role="cell"><PresenceLabel online={online} /></span>
    </div>
  )
})
