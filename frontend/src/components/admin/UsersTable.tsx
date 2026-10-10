import { memo, useRef, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import type { Member } from '../../api/admin'
import { rolePills, statusLabel } from '../../lib/admin-model'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import { Avatar, Pressable } from '../primitives'
import { StatusPill } from '../approval/StatusPill'

/**
 * Thành viên · phòng ban · vai trò · trạng thái. The columns follow the table's
 * own width, not the window's: with the panel open the table is narrow, so the
 * status goes first (as on a tablet), then the role and department move under
 * the name (as on a phone).
 */
const COLS = `grid grid-cols-[minmax(0,1fr)_auto]
  @xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_minmax(0,1.2fr)]
  @3xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_minmax(0,1.2fr)_9.5rem]
  gap-x-3 items-center px-2.5`

interface UsersTableProps {
  /** Table name for assistive tech. */
  label: string
  members: Member[]
  loading: boolean
  /** Person open in the panel (their node, never shown). */
  openId: string | null
  onOpen: (m: Member) => void
  /** Changes with the filters; rows enter in sequence when the list is new. */
  scope: string
}

const TONE: Record<string, 'ok' | 'wait' | 'bad'> = { active: 'ok', invited: 'wait', disabled: 'bad' }

/**
 * The hairline table (DESIGN.md §6; admin density 7 is the same 44px rows with
 * tighter gaps): header 12px/600, 1px lines, no zebra. A person's name is the
 * real control, a button stretched over the whole row, so the row is reachable
 * by keyboard and ↑/↓ move between rows.
 */
export function UsersTable({ label, members, loading, openId, onOpen, scope }: UsersTableProps) {
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)

  // Rows on screen when the list first shows enter in sequence; later arrivals enter alone.
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && members.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(members.map((p, n) => [p.ngac_node_id, n])) }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const items = Array.from(bodyRef.current?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
    const at = items.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    const next = items[e.key === 'ArrowDown' ? Math.min(at + 1, items.length - 1) : Math.max(at - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  return (
    <div role="table" aria-label={label} aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Thành viên</span>
        <span role="columnheader" className="hidden @xl:block">Phòng ban</span>
        <span role="columnheader" className="hidden @xl:block">Vai trò</span>
        <span role="columnheader" className="@xl:hidden @3xl:block text-right @3xl:text-left">Trạng thái</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3, 4].map((i) => (
            <div key={i} role="row" className={`${COLS} h-11 border-b border-line`}>
              <span className="flex items-center gap-2.5">
                <span className="skeleton w-6 h-6 rounded-full" />
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${70 - i * 8}%` }} />
              </span>
              <span className="skeleton h-3.5 w-2/3 rounded-sm hidden @xl:block" />
              <span className="skeleton h-5 w-24 rounded-full hidden @xl:block" />
              <span className="skeleton h-5.5 w-24 rounded-full justify-self-end @xl:hidden @3xl:block @3xl:justify-self-start" />
            </div>
          ))
        ) : (
          <AnimatePresence key={scope} initial={false}>
            {members.map((p) => {
              const index = firstBatch.current?.ids.get(p.ngac_node_id)
              const first = index !== undefined
              const delay = first ? staggerDelay(index, m.reduced) : 0
              return (
                <motion.div
                  key={p.ngac_node_id}
                  {...withDelay(first ? m.route : m.row, delay)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <UserRow member={p} open={openId === p.ngac_node_id} onOpen={onOpen} />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

const UserRow = memo(function UserRow({ member, open, onOpen }: { member: Member; open: boolean; onOpen: (m: Member) => void }) {
  const pills = rolePills(member)
  const status = { tone: TONE[member.status] ?? 'ok', label: statusLabel(member.status) } as const
  const sub = [member.department?.name, pills.shown.join(', ')].filter(Boolean).join(' · ')
  return (
    <div
      role="row"
      className={`${COLS} row-lazy relative min-h-11 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0 py-1.5">
        <Avatar name={member.display_name} hueKey={member.user_id} src={member.avatar_url} size={24} />
        <span className="grid min-w-0">
          <Pressable
            data-row
            aria-current={open || undefined}
            onClick={() => onOpen(member)}
            className="min-w-0 truncate font-medium text-ink text-left cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {member.display_name}
          </Pressable>
          {member.email && <small className="text-xs text-ink-muted truncate">{member.email}</small>}
          <small className="text-xs text-ink-muted truncate @xl:hidden">{sub}</small>
        </span>
      </span>
      <span role="cell" className="hidden @xl:block truncate text-ink">
        {member.department?.name ?? <span className="text-ink-muted">Chưa có</span>}
      </span>
      <span role="cell" className="hidden @xl:flex items-center gap-1 min-w-0">
        {pills.shown.map((r) => (
          <span key={r} className="inline-flex items-center min-w-0 h-5.5 px-2 rounded-full bg-sunk text-xs font-semibold text-ink">
            <span className="truncate">{r}</span>
          </span>
        ))}
        {pills.more > 0 && <span className="text-xs font-semibold text-ink-muted tnum">+{pills.more}</span>}
      </span>
      <span role="cell" className="flex justify-end @xl:hidden @3xl:flex @3xl:justify-start">
        <StatusPill pill={status} />
      </span>
    </div>
  )
})
