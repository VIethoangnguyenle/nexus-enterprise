import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { Building2, CircleAlert, KeyRound, Mail, Plus, UserPlus, Users } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCreateDepartment, useDepartments, useInvitations, useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import { childrenIndex, deptDepth } from '../../lib/admin-model'
import { openSearch, type AdminSearch } from '../../lib/admin-search'
import { formatCount } from '../../lib/format'
import { useMotionPresets } from '../../lib/motion'
import { Button, Heading, toast } from '../primitives'
import { TreeView, type TreeNode } from '../composites/TreeView'
import { EmptyState } from '../spaces/EmptyState'
import { AdminFrame } from './AdminFrame'
import { DepartmentDialog } from './DepartmentDialog'
import { DepartmentPanel } from './DepartmentPanel'
import { InviteDialog } from './InviteDialog'

const treeIcon = () => <Building2 size={16} strokeWidth={1.75} className="text-accent shrink-0" aria-hidden="true" />

/**
 * Tổng quan (design/mockups/admin.html §1): three figures and the organisation
 * as one tree. Choosing a department opens it in the panel on the right; which
 * one is open lives in the URL, so reload and Back keep it.
 *
 * What the mockup also shows, an administrators' activity feed, has no source
 * to read from yet (nothing records who changed a role, a permission or a
 * department), so it is left out rather than drawn from invented lines.
 */
export function OverviewScreen() {
  const search = useSearch({ strict: false }) as AdminSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const { workspaceId: wsId } = useActiveWorkspace()
  const departments = useDepartments(wsId)
  const people = useMemberDirectory(wsId)
  const roles = useRoles(wsId)
  const invitations = useInvitations(wsId)
  const create = useCreateDepartment(wsId)

  const [inviting, setInviting] = useState(false)
  const [creating, setCreating] = useState(false)
  const [open, setOpen] = useState<Set<string> | null>(null)

  const flat = useMemo(() => departments.data?.flat ?? [], [departments.data])
  const index = useMemo(() => childrenIndex(flat), [flat])
  const roots = useMemo<TreeNode[]>(
    () => (index.get('') ?? []).map((d) => ({ id: d.id, label: d.name, hasChildren: (index.get(d.id) ?? []).length > 0 })),
    [index],
  )
  const useChildren = useCallback(
    (parentId: string) => ({
      nodes: (index.get(parentId) ?? []).map((d) => ({ id: d.id, label: d.name, hasChildren: (index.get(d.id) ?? []).length > 0 })),
      isLoading: false,
      isError: false,
    }),
    [index],
  )
  // The first two levels start open, so the organisation is visible without clicking.
  const initiallyOpen = useMemo(
    () => new Set(roots.flatMap((r) => [r.id, ...(index.get(r.id) ?? []).map((c) => c.id)])),
    [roots, index],
  )
  const expanded = open ?? initiallyOpen
  const toggle = useCallback(
    (id: string) => setOpen((cur) => {
      const next = new Set(cur ?? initiallyOpen)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    }),
    [initiallyOpen],
  )
  const counts = useMemo(() => new Map(flat.map((d) => [d.id, d.member_count])), [flat])

  const go = useCallback(
    (to: (prev: AdminSearch) => AdminSearch) => void navigate({ to: '/admin', search: to }),
    [navigate],
  )
  const openId = search.dept ?? null
  const openDept = flat.find((d) => d.id === openId)
  const closePanel = useCallback(() => go((p) => openSearch(p, 'dept')), [go])

  // Esc closes the panel. Dialogs and pickers claim the key first
  // (preventDefault on `document`); listening on `window` puts this after
  // them, so one Esc closes one thing.
  useEffect(() => {
    if (!openDept) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) closePanel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [openDept, closePanel])

  const members = people.data ?? []
  const active = members.filter((p) => p.status === 'active').length
  const customRoles = roles.data?.custom ?? []
  // An offer expiring within two days is worth a nudge.
  const expiringSoon = (invitations.data ?? []).filter((i) => new Date(i.expires_at).getTime() - Date.now() < 2 * 24 * 3600 * 1000).length
  const systemRoles = roles.data?.system ?? []

  const inviteButton = (
    <Button size="sm" onClick={() => setInviting(true)}>
      <UserPlus size={16} strokeWidth={1.75} aria-hidden="true" />
      <span className="max-sm:sr-only">Mời thành viên</span>
    </Button>
  )

  return (
    <AdminFrame section="overview" subtitle={people.data ? `${formatCount(members.length)} thành viên` : undefined} action={inviteButton}>
      <div className="relative flex flex-1 min-h-0 min-w-0">
        <div className="flex-1 min-w-0 min-h-0 overflow-y-auto">
          <div className="grid gap-4 px-5 pt-3 pb-5 content-start">
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
              <Stat
                icon={<Users size={16} strokeWidth={1.75} />}
                label="Thành viên"
                loading={people.isPending}
                value={people.data ? formatCount(members.length) : undefined}
                sub={`${formatCount(active)} đang hoạt động`}
              />
              <Stat
                icon={<Building2 size={16} strokeWidth={1.75} />}
                label="Phòng ban"
                loading={departments.isPending}
                value={departments.data ? formatCount(flat.length) : undefined}
                sub={`${deptDepth(flat)} cấp`}
              />
              {invitations.data && (
                <Stat
                  icon={<Mail size={16} strokeWidth={1.75} />}
                  label="Lời mời chờ nhận"
                  loading={false}
                  value={formatCount(invitations.data.length)}
                  sub={expiringSoon > 0 ? `${expiringSoon} sắp hết hạn` : 'Chưa có lời mời nào sắp hết hạn'}
                />
              )}
              <Stat
                icon={<KeyRound size={16} strokeWidth={1.75} />}
                label="Vai trò"
                loading={roles.isPending}
                value={roles.data ? formatCount(customRoles.length + systemRoles.length) : undefined}
                sub={`${systemRoles.length} hệ thống, ${customRoles.length} tuỳ chỉnh`}
              />
            </div>

            <section aria-label="Sơ đồ tổ chức" className="grid gap-2 p-4 rounded-surface bg-raised content-start">
              <div className="flex items-center gap-2">
                <Heading as="h2" look="section">Sơ đồ tổ chức</Heading>
                <Button variant="ghost" size="sm" className="ml-auto" onClick={() => setCreating(true)}>
                  <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                  Phòng ban
                </Button>
              </div>
              {departments.isPending ? (
                <div className="grid gap-1.5" aria-busy="true" aria-label="Đang tải sơ đồ tổ chức">
                  {[78, 64, 52, 70].map((w, i) => <div key={i} className="skeleton h-7 rounded-sm" style={{ width: `${w}%` }} />)}
                </div>
              ) : departments.isError ? (
                <EmptyState
                  compact
                  icon={<CircleAlert size={24} strokeWidth={1.75} />}
                  text="Không tải được sơ đồ tổ chức. Kiểm tra kết nối rồi thử lại."
                  action={<Button variant="soft" size="sm" onClick={() => void departments.refetch()}>Thử lại</Button>}
                />
              ) : flat.length === 0 ? (
                <EmptyState
                  compact
                  icon={<Building2 size={24} strokeWidth={1.75} />}
                  text="Chưa có phòng ban nào. Tạo phòng ban để sắp xếp mọi người theo cơ cấu của khối."
                  action={<Button size="sm" onClick={() => setCreating(true)}>Tạo phòng ban</Button>}
                />
              ) : (
                <motion.div {...m.route}>
                  <TreeView
                    label="Phòng ban"
                    roots={roots}
                    useChildren={useChildren}
                    expanded={expanded}
                    onToggle={toggle}
                    selectedId={openId}
                    onSelect={(id) => go((p) => openSearch(p, 'dept', id))}
                    icon={treeIcon}
                    trailing={(n) => formatCount(counts.get(n.id) ?? 0)}
                    emptyLabel="Không có phòng ban con"
                  />
                </motion.div>
              )}
            </section>
          </div>
        </div>

        <AnimatePresence>
          {openDept && (
            <DepartmentPanel
              key={openDept.id}
              workspaceId={wsId}
              dept={openDept}
              departments={flat}
              members={members}
              onClose={closePanel}
            />
          )}
        </AnimatePresence>
      </div>

      <InviteDialog open={inviting} onClose={() => setInviting(false)} workspaceId={wsId} roles={customRoles} departments={flat} />
      <DepartmentDialog
        key={creating ? `new-${openId ?? ''}` : 'closed'}
        open={creating}
        onClose={() => setCreating(false)}
        departments={flat}
        initialParent={openId ?? ''}
        pending={create.isPending}
        onSubmit={(name, parentId) =>
          create.mutate(
            { name, parentId: parentId || undefined },
            { onSuccess: () => { setCreating(false); toast(`Đã tạo phòng ${name}`) } },
          )
        }
      />
    </AdminFrame>
  )
}

function Stat({ icon, label, value, sub, loading }: { icon: ReactNode; label: string; value?: string; sub: string; loading: boolean }) {
  return (
    <div className="grid gap-1 p-4 rounded-surface bg-raised content-start" aria-busy={loading || undefined}>
      <span className="flex items-center gap-1.5 text-sm text-ink-muted">
        <span aria-hidden="true">{icon}</span>
        {label}
      </span>
      {loading ? (
        <div className="skeleton h-8 w-16 rounded-sm" />
      ) : (
        <span className="font-display text-2xl font-semibold tnum">{value ?? '?'}</span>
      )}
      <span className="text-xs text-ink-muted">{loading ? ' ' : value ? sub : 'Không tải được'}</span>
    </div>
  )
}
