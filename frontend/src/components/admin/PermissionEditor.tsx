import { useMemo, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { ChevronRight, CircleAlert, Lock, TriangleAlert } from 'lucide-react'
import type { PermissionArea } from '../../api/admin'
import { usePermissionAreas, useRole, useSaveRolePermissions } from '../../hooks/useAdmin'
import { usePhone } from '../../hooks/usePhone'
import {
  changeCount, changesBetween, consequenceLine, grantsOf, roleName, setOp, type Grants,
} from '../../lib/admin-model'
import { statusOf } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { Button, Heading, Pressable, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { PermissionMatrix } from './PermissionMatrix'
import { PermissionSwitches } from './PermissionSwitches'

interface PermissionEditorProps {
  workspaceId: string
  roleId: string
  /** Leave the editor (saved, cancelled, or nothing to edit). */
  onDone: () => void
}

/**
 * Sửa quyền của vai trò (design/mockups/admin.html §5): the matrix of areas ×
 * operations, or on a phone a list of switches. Both are drawn from what the
 * server says each area offers, so an operation an area does not support is not
 * there to press. Changes are drafted locally; the bar below says how many and
 * who they affect, and saving writes one area at a time. The role is read afresh
 * when it opens (mount it keyed by the role), so the draft starts from the
 * server's truth.
 *
 * The two built-in roles are not editable; a link to one lands on a note.
 */
export function PermissionEditor({ workspaceId, roleId, onDone }: PermissionEditorProps) {
  const m = useMotionPresets()
  const phone = usePhone()
  const detail = useRole(workspaceId, roleId)
  const areasQ = usePermissionAreas(workspaceId)
  const save = useSaveRolePermissions(workspaceId, roleId)
  const [draft, setDraft] = useState<Grants | null>(null)

  const role = detail.data?.role
  const areas = useMemo<PermissionArea[]>(() => areasQ.data ?? [], [areasQ.data])
  const saved = useMemo(() => grantsOf(detail.data?.permissions ?? []), [detail.data])
  const current = draft ?? saved
  const changes = useMemo(() => changesBetween(saved, current, areas), [saved, current, areas])
  const changed = useMemo(() => new Set(changes.map((c) => c.area)), [changes])
  const name = role ? roleName(role) : ''

  const crumbs = (
    <nav aria-label="Đường dẫn" className="flex items-center gap-1 px-5 pt-3 text-sm text-ink-muted min-w-0">
      <Button variant="link" size="link" onClick={onDone}>Vai trò</Button>
      {name && (
        <>
          <ChevronRight size={14} strokeWidth={1.75} aria-hidden="true" />
          <span className="truncate">{name}</span>
        </>
      )}
    </nav>
  )

  if (detail.isPending || areasQ.isPending) {
    return (
      <div className="flex-1 min-w-0">
        {crumbs}
        <div className="grid gap-3 px-5 pt-4" aria-busy="true" aria-label="Đang tải quyền">
          {[0, 1, 2, 3].map((i) => <div key={i} className="skeleton h-12 rounded-sm" />)}
        </div>
      </div>
    )
  }
  if (detail.isError || areasQ.isError || !role) {
    const forbidden = statusOf(detail.error) === 403 || statusOf(areasQ.error) === 403
    const gone = statusOf(detail.error) === 404
    return (
      <div className="flex-1 min-w-0">
        {crumbs}
        <EmptyState
          icon={forbidden ? <Lock size={24} strokeWidth={1.75} /> : <CircleAlert size={24} strokeWidth={1.75} />}
          text={
            forbidden ? 'Bạn không có quyền sửa quyền của vai trò này.'
            : gone ? 'Vai trò này không còn nữa.'
            : 'Không tải được quyền của vai trò. Kiểm tra kết nối rồi thử lại.'
          }
          action={
            gone || forbidden ? <Button variant="soft" size="sm" onClick={onDone}>Về danh sách vai trò</Button>
            : <Button variant="soft" size="sm" onClick={() => { void detail.refetch(); void areasQ.refetch() }}>Thử lại</Button>
          }
        />
      </div>
    )
  }
  if (role.kind !== 'custom') {
    return (
      <div className="flex-1 min-w-0">
        {crumbs}
        <EmptyState
          icon={<Lock size={24} strokeWidth={1.75} />}
          text={`${name} là vai trò hệ thống, không sửa được quyền.`}
          action={<Button variant="soft" size="sm" onClick={onDone}>Về danh sách vai trò</Button>}
        />
      </div>
    )
  }

  const toggle = (area: PermissionArea, op: string, on: boolean) => setDraft((cur) => setOp(cur ?? saved, area, op, on))
  const n = changeCount(changes)
  const effect = consequenceLine(name, role.member_count, changes)

  const submit = () => {
    const payload = changes.map((c) => ({ area: c.area, operations: current[c.area] ?? [] }))
    const before = changes.map((c) => ({ area: c.area, operations: saved[c.area] ?? [] }))
    save.mutate(payload, {
      onSuccess: () => {
        setDraft(null)
        toast(`Đã lưu quyền của ${name}`, {
          action: { label: 'Hoàn tác', onClick: () => save.mutate(before) },
        })
        onDone()
      },
      // A partial save leaves some areas written: show what the server holds.
      onError: () => setDraft(null),
    })
  }

  const props = { areas, draft: current, changed, role: name, onToggle: toggle, disabled: save.isPending }

  return (
    <div className="flex-1 flex flex-col min-w-0 min-h-0">
      {crumbs}
      <div className="px-5 pt-1 pb-2">
        <Heading as="h2" look="page">Sửa quyền</Heading>
        <p className="m-0 text-sm text-ink-muted">Chọn thao tác mỗi vùng cho phép. Thay đổi chỉ có hiệu lực sau khi lưu.</p>
      </div>
      <div className="flex-1 min-h-0 overflow-y-auto px-3 pb-4">
        {areas.length === 0 ? (
          <EmptyState icon={<CircleAlert size={24} strokeWidth={1.75} />} text="Workspace này chưa có vùng nào để cấp quyền." />
        ) : phone ? (
          <PermissionSwitches {...props} />
        ) : (
          <PermissionMatrix {...props} />
        )}
      </div>

      <AnimatePresence>
        {n > 0 && (
          <motion.div
            key="bar"
            role="region"
            aria-label="Thay đổi chưa lưu"
            {...m.toast}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 mx-3 mb-3 px-4 py-3 rounded-overlay bg-overlay shadow-overlay"
          >
            <TriangleAlert size={18} strokeWidth={1.75} className="text-warning shrink-0" aria-hidden="true" />
            <p className="m-0 flex-1 min-w-48 text-sm">
              <b className="font-semibold">{n} thay đổi chưa lưu.</b> {effect}
            </p>
            <Pressable onClick={() => setDraft(null)} disabled={save.isPending} className="h-9 px-3 rounded-md text-sm font-semibold hover:bg-hover">
              Huỷ
            </Pressable>
            <Button size="sm" loading={save.isPending} onClick={submit}>Lưu</Button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
