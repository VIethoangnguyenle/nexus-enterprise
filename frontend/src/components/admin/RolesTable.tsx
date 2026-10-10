import { memo, useRef, type KeyboardEvent } from 'react'
import { Crown, KeyRound, Lock, Users } from 'lucide-react'
import type { RoleSummary } from '../../api/admin'
import { roleLine, roleName } from '../../lib/admin-model'
import { formatCount } from '../../lib/format'
import { Pressable } from '../primitives'

const COLS = `grid grid-cols-[minmax(0,1fr)_auto] @xl:grid-cols-[minmax(0,1fr)_8rem_6rem]
  gap-x-3 items-center px-2.5`

const ICON = { owners: Crown, members: Users, custom: KeyRound } as const

interface RolesTableProps {
  label: string
  /** Built-in roles first, then the administrator's. */
  roles: RoleSummary[]
  loading: boolean
  openId: string | null
  onOpen: (r: RoleSummary) => void
}

/**
 * Vai trò · loại · thành viên. A role is read by its name (the display name an
 * administrator typed, or "Chủ sở hữu" / "Thành viên" for the two built-in
 * ones); a built-in role carries a lock and the word "Hệ thống". The name is the
 * real control, a button stretched over the row; ↑/↓ move between rows.
 */
export function RolesTable({ label, roles, loading, openId, onOpen }: RolesTableProps) {
  const bodyRef = useRef<HTMLDivElement>(null)

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
        <span role="columnheader">Vai trò</span>
        <span role="columnheader" className="hidden @xl:block">Loại</span>
        <span role="columnheader" className="text-right">Thành viên</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading
          ? [0, 1, 2, 3].map((i) => (
              <div key={i} role="row" className={`${COLS} h-14 border-b border-line`}>
                <span className="flex items-center gap-2.5">
                  <span className="skeleton w-8 h-8 rounded-surface" />
                  <span className="grid gap-1.5 flex-1">
                    <span className="skeleton h-3.5 rounded-sm" style={{ width: `${60 - i * 8}%` }} />
                    <span className="skeleton h-3 w-1/3 rounded-sm" />
                  </span>
                </span>
                <span className="skeleton h-3.5 w-16 rounded-sm hidden @xl:block" />
                <span className="skeleton h-3.5 w-8 rounded-sm justify-self-end" />
              </div>
            ))
          : roles.map((r) => <RoleRow key={r.id} role={r} open={openId === r.id} onOpen={onOpen} />)}
      </div>
    </div>
  )
}

const RoleRow = memo(function RoleRow({ role, open, onOpen }: { role: RoleSummary; open: boolean; onOpen: (r: RoleSummary) => void }) {
  const Icon = ICON[role.kind]
  const system = role.kind !== 'custom'
  return (
    <div
      role="row"
      className={`${COLS} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0 py-2">
        <span className="grid place-items-center w-8 h-8 rounded-surface bg-accent-wash text-accent shrink-0" aria-hidden="true">
          <Icon size={16} strokeWidth={1.75} />
        </span>
        <span className="grid min-w-0">
          <Pressable
            data-row
            aria-current={open || undefined}
            onClick={() => onOpen(role)}
            className="min-w-0 truncate font-medium text-ink text-left cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {roleName(role)}
          </Pressable>
          <small className="text-xs text-ink-muted truncate">{roleLine(role)}</small>
        </span>
      </span>
      <span role="cell" className="hidden @xl:flex items-center gap-1.5 text-ink">
        {system && <Lock size={14} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />}
        {system ? 'Hệ thống' : 'Tuỳ chỉnh'}
      </span>
      <span role="cell" className="text-right tnum">{formatCount(role.member_count)}</span>
    </div>
  )
})
