import { useMemo, useState } from 'react'
import { Lock, Trash2, X } from 'lucide-react'
import type { Department, Member, RoleSummary } from '../../api/admin'
import { useAssignRole, useRemoveMember, useUnassignRole, useUpdateMemberDepartment } from '../../hooks/useAdmin'
import { MEMBER_LABEL, OWNER_LABEL, roleName, statusLabel } from '../../lib/admin-model'
import { Avatar, Button, IconButton, toast } from '../primitives'
import { StatusPill } from '../approval/StatusPill'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { SidePanel } from '../spaces/SidePanel'
import { DepartmentPicker } from './DepartmentPicker'
import { RolePicker } from './RolePicker'

const TONE = { active: 'ok', invited: 'wait', disabled: 'bad' } as const

interface UserPanelProps {
  workspaceId: string
  member: Member
  /** The administrator's own roles, for adding. */
  roles: RoleSummary[]
  departments: Department[]
  /** The signed-in person: they do not remove themselves from here. */
  isMe: boolean
  onClose: () => void
}

/**
 * One person (design/mockups/admin.html §2): their roles as pills that can be
 * removed in place with an undo, a picker to add one, their department chosen
 * from the tree, and removal from the workspace. Mount it keyed by the person,
 * so a dialog never carries over to the next one.
 *
 * Locking an account is left out: nothing in the backend can lock one.
 */
export function UserPanel({ workspaceId, member, roles, departments, isMe, onClose }: UserPanelProps) {
  const assign = useAssignRole(workspaceId)
  const unassign = useUnassignRole(workspaceId)
  const setDepartment = useUpdateMemberDepartment(workspaceId)
  const remove = useRemoveMember(workspaceId)
  const [confirming, setConfirming] = useState(false)

  const held = useMemo(() => new Set(member.roles.map((r) => r.id)), [member.roles])
  const available = useMemo(() => roles.filter((r) => !held.has(r.id)), [roles, held])
  const sub = [member.title, member.email].filter(Boolean).join(' · ')
  const first = member.display_name

  const removeRole = (id: string, name: string) =>
    unassign.mutate(
      { nodeId: member.ngac_node_id, roleId: id },
      {
        onSuccess: () =>
          toast(`Đã bỏ vai trò ${name} của ${first}`, {
            action: { label: 'Hoàn tác', onClick: () => assign.mutate({ nodeId: member.ngac_node_id, roleId: id }) },
          }),
      },
    )

  return (
    <SidePanel
      label="Thành viên"
      title={member.display_name}
      sub={sub}
      closeLabel="thành viên"
      onClose={onClose}
      footer={
        !isMe && (
          <div className="flex items-center gap-2 px-4 pt-2 pb-4">
            <Button variant="ghost" size="sm" className="text-danger" onClick={() => setConfirming(true)}>
              <Trash2 size={16} strokeWidth={1.75} aria-hidden="true" />
              Xoá khỏi workspace
            </Button>
          </div>
        )
      }
    >
      <div className="grid gap-5 px-3 pt-1 pb-3 min-w-0">
        <div className="flex items-center gap-3">
          <Avatar name={member.display_name} hueKey={member.user_id} src={member.avatar_url} size={40} />
          <StatusPill pill={{ tone: TONE[member.status] ?? 'ok', label: statusLabel(member.status) }} />
        </div>

        <section className="grid gap-2 min-w-0">
          <h3 className="m-0 text-label text-ink-muted">Vai trò</h3>
          <ul aria-label="Vai trò của thành viên" className="flex flex-wrap items-center gap-1.5 m-0 p-0 list-none">
            {member.is_owner && <FixedPill name={OWNER_LABEL} />}
            {!member.is_owner && member.roles.length === 0 && <FixedPill name={MEMBER_LABEL} />}
            {member.roles.map((r) => (
              <li key={r.id}>
                <span className="inline-flex items-center gap-1 h-7 pl-3 pr-1 rounded-full bg-sunk text-small-ui text-ink">
                  {r.name}
                  <IconButton size="sm" aria-label={`Bỏ vai trò ${r.name}`} disabled={unassign.isPending} onClick={() => removeRole(r.id, r.name)}>
                    <X size={14} strokeWidth={1.75} />
                  </IconButton>
                </span>
              </li>
            ))}
          </ul>
          <RolePicker
            label="Thêm vai trò"
            roles={available}
            disabled={assign.isPending}
            emptyText={roles.length === 0 ? 'Chưa có vai trò tuỳ chỉnh nào. Tạo ở thẻ Vai trò.' : 'Người này đã có mọi vai trò.'}
            onPick={(r) =>
              assign.mutate(
                { nodeId: member.ngac_node_id, roleId: r.id },
                { onSuccess: () => toast(`Đã gán vai trò ${roleName(r)} cho ${first}`) },
              )
            }
          />
        </section>

        <section className="grid gap-2 min-w-0">
          <h3 className="m-0 text-label text-ink-muted">Phòng ban</h3>
          <DepartmentPicker
            label="Phòng ban"
            departments={departments}
            value={member.department?.id ?? ''}
            noneLabel="Chưa thuộc phòng ban nào"
            disabled={setDepartment.isPending}
            onChange={(id) => {
              if (id === (member.department?.id ?? '')) return
              const name = departments.find((d) => d.id === id)?.name
              setDepartment.mutate(
                { nodeId: member.ngac_node_id, departmentId: id },
                { onSuccess: () => toast(name ? `Đã chuyển ${first} sang phòng ${name}` : `Đã đưa ${first} ra khỏi phòng ban`) },
              )
            }}
          />
        </section>
      </div>

      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        title={`Xoá ${member.display_name} khỏi workspace?`}
        description="Người này mất quyền vào mọi thứ trong workspace. Họ vẫn giữ tài khoản và có thể được mời lại."
        confirmLabel="Xoá khỏi workspace"
        confirmVariant="danger"
        loading={remove.isPending}
        onConfirm={() =>
          remove.mutate(member.ngac_node_id, {
            onSuccess: () => {
              setConfirming(false)
              toast(`Đã xoá ${member.display_name} khỏi workspace`)
              onClose()
            },
            onError: () => setConfirming(false),
          })
        }
      />
    </SidePanel>
  )
}

/** A role that cannot be taken off here: the built-in ones. */
function FixedPill({ name }: { name: string }) {
  return (
    <li>
      <span className="inline-flex items-center gap-1.5 h-7 px-3 rounded-full bg-sunk text-small-ui text-ink">
        {name}
        <Lock size={14} strokeWidth={1.75} className="text-ink-muted" aria-label="Không đổi được ở đây" />
      </span>
    </li>
  )
}
