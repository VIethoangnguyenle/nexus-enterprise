import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { Check, ChevronDown, CircleAlert, UserPlus, Users } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useDepartments, useInvitations, useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import { deptPathLabel, deptWithDescendants, matchesMember, OWNER_LABEL, roleName } from '../../lib/admin-model'
import { openSearch, type AdminSearch } from '../../lib/admin-search'
import { formatCount } from '../../lib/format'
import { useMotionPresets } from '../../lib/motion'
import { useAuthStore } from '../../stores/auth.store'
import { Button, MenuItem, MenuSeparator, Popover, SearchField } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { AdminFrame } from './AdminFrame'
import { InvitationsList } from './InvitationsList'
import { InviteDialog } from './InviteDialog'
import { UserPanel } from './UserPanel'
import { UsersTable } from './UsersTable'

const ALL_ROLES = ''
const OWNERS = 'owners'

/**
 * Người dùng (design/mockups/admin.html §2): everyone in the workspace as a
 * hairline table with search and two filters, and one person at a time in the
 * panel. Who is open is part of the URL.
 */
export function UsersScreen() {
  const search = useSearch({ strict: false }) as AdminSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const me = useAuthStore((s) => s.user)
  const { workspaceId: wsId } = useActiveWorkspace()
  const people = useMemberDirectory(wsId)
  const departments = useDepartments(wsId)
  const roles = useRoles(wsId)
  const invitations = useInvitations(wsId)

  const [inviting, setInviting] = useState(false)
  const [query, setQuery] = useState('')
  const [deptFilter, setDeptFilter] = useState('')
  const [roleFilter, setRoleFilter] = useState(ALL_ROLES)

  const flat = useMemo(() => departments.data?.flat ?? [], [departments.data])
  const customRoles = useMemo(() => roles.data?.custom ?? [], [roles.data])
  const everyone = useMemo(() => people.data ?? [], [people.data])

  const filtered = useMemo(() => {
    const inDept = deptFilter ? deptWithDescendants(flat, deptFilter) : null
    return everyone.filter(
      (p) =>
        matchesMember(p, query) &&
        (!inDept || (p.department && inDept.has(p.department.id))) &&
        (roleFilter === ALL_ROLES || (roleFilter === OWNERS ? p.is_owner : p.roles.some((r) => r.id === roleFilter))),
    )
  }, [everyone, query, deptFilter, roleFilter, flat])
  const filtering = !!query.trim() || !!deptFilter || !!roleFilter

  const go = useCallback(
    (to: (prev: AdminSearch) => AdminSearch) => void navigate({ to: '/admin/users', search: to }),
    [navigate],
  )
  const openId = search.member ?? null
  const openMember = everyone.find((p) => p.ngac_node_id === openId)
  const closePanel = useCallback(() => go((p) => openSearch(p, 'member')), [go])

  // Esc closes the panel; dialogs and pickers claim the key first.
  useEffect(() => {
    if (!openMember) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) closePanel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [openMember, closePanel])

  const pending = invitations.data ?? []
  const pendingCount = pending.length
  const loading = people.isPending
  const failed = people.isError && !people.data

  const deptLabel = deptFilter ? deptPathLabel(flat, deptFilter) : 'Phòng ban'
  const roleLabel = roleFilter === OWNERS ? OWNER_LABEL : customRoles.find((r) => r.id === roleFilter) ? roleName(customRoles.find((r) => r.id === roleFilter)!) : 'Vai trò'

  const inviteButton = (
    <Button size="sm" onClick={() => setInviting(true)}>
      <UserPlus size={16} strokeWidth={1.75} aria-hidden="true" />
      <span className="max-sm:sr-only">Mời thành viên</span>
    </Button>
  )

  const body = (() => {
    if (failed) {
      return (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được danh sách thành viên. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void people.refetch()}>Thử lại</Button>}
        />
      )
    }
    if (!loading && everyone.length === 0) {
      return (
        <EmptyState
          icon={<Users size={24} strokeWidth={1.75} />}
          text="Workspace chưa có thành viên nào. Mời người đã có tài khoản bằng email."
          action={inviteButton}
        />
      )
    }
    if (!loading && filtered.length === 0) {
      return (
        <EmptyState
          icon={<Users size={24} strokeWidth={1.75} />}
          text="Không có ai khớp với bộ lọc. Bỏ bớt điều kiện để xem thêm."
          action={
            <Button variant="soft" size="sm" onClick={() => { setQuery(''); setDeptFilter(''); setRoleFilter(ALL_ROLES) }}>
              Xoá bộ lọc
            </Button>
          }
        />
      )
    }
    return (
      <motion.div {...m.route}>
        <UsersTable
          label="Thành viên"
          members={filtered}
          loading={loading}
          openId={openId}
          onOpen={(p) => go((prev) => openSearch(prev, 'member', p.ngac_node_id))}
          scope={`${deptFilter}|${roleFilter}|${query}`}
        />
      </motion.div>
    )
  })()

  return (
    <AdminFrame
      section="users"
      subtitle={people.data ? `${formatCount(everyone.length)} thành viên${pendingCount > 0 ? `, ${formatCount(pendingCount)} lời mời` : ''}` : undefined}
      action={inviteButton}
    >
      <div className="relative flex flex-1 min-h-0 min-w-0">
        <div className="flex-1 flex flex-col min-w-0 min-h-0">
          <div className="flex flex-wrap items-center gap-2 px-5 pt-3 pb-2">
            <SearchField
              label="Tìm theo tên hoặc email"
              moduleSearch
              tone="sunk"
              className="w-full sm:w-64"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <FilterMenu label="Lọc theo phòng ban" text={deptLabel} active={!!deptFilter}>
              <MenuItem aria-current={(!deptFilter) || undefined} onClick={() => setDeptFilter('')} icon={<Tick on={!deptFilter} />}>
                Tất cả phòng ban
              </MenuItem>
              <MenuSeparator />
              {flat.map((d) => (
                <MenuItem key={d.id} aria-current={(deptFilter === d.id) || undefined} onClick={() => setDeptFilter(d.id)} icon={<Tick on={deptFilter === d.id} />}>
                  {deptPathLabel(flat, d.id)}
                </MenuItem>
              ))}
            </FilterMenu>
            <FilterMenu label="Lọc theo vai trò" text={roleLabel} active={!!roleFilter}>
              <MenuItem aria-current={(!roleFilter) || undefined} onClick={() => setRoleFilter(ALL_ROLES)} icon={<Tick on={!roleFilter} />}>
                Tất cả vai trò
              </MenuItem>
              <MenuSeparator />
              <MenuItem aria-current={(roleFilter === OWNERS) || undefined} onClick={() => setRoleFilter(OWNERS)} icon={<Tick on={roleFilter === OWNERS} />}>
                {OWNER_LABEL}
              </MenuItem>
              {customRoles.map((r) => (
                <MenuItem key={r.id} aria-current={(roleFilter === r.id) || undefined} onClick={() => setRoleFilter(r.id)} icon={<Tick on={roleFilter === r.id} />}>
                  {roleName(r)}
                </MenuItem>
              ))}
            </FilterMenu>
            {filtering && !loading && (
              <span role="status" className="text-sm text-ink-muted tnum">{formatCount(filtered.length)} người</span>
            )}
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto">
            {body}
            {!filtering && !loading && <InvitationsList workspaceId={wsId} invitations={pending} />}
          </div>
        </div>

        <AnimatePresence>
          {openMember && (
            <UserPanel
              key={openMember.ngac_node_id}
              workspaceId={wsId}
              member={openMember}
              roles={customRoles}
              departments={flat}
              isMe={openMember.ngac_node_id === me?.ngac_node_id}
              onClose={closePanel}
            />
          )}
        </AnimatePresence>
      </div>

      <InviteDialog open={inviting} onClose={() => setInviting(false)} workspaceId={wsId} roles={customRoles} departments={flat} />
    </AdminFrame>
  )
}

/** A filter: a button that opens a menu of choices; it says what it is set to. */
function FilterMenu({ label, text, active, children }: { label: string; text: string; active: boolean; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const anchor = useRef<HTMLButtonElement>(null)
  return (
    <>
      <Button
        ref={anchor}
        variant={active ? 'tonal' : 'soft'}
        size="sm"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="max-w-56"
      >
        <span className="truncate">{text}</span>
        <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
      </Button>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={anchor} role="menu" label={label} className="max-h-80 overflow-y-auto">
        {children}
      </Popover>
    </>
  )
}

const Tick = ({ on }: { on: boolean }) => (
  <Check size={16} strokeWidth={1.75} aria-hidden="true" className={on ? 'text-accent' : 'opacity-0'} />
)
