import { useState } from 'react'
import { CircleAlert, Lock, Trash2 } from 'lucide-react'
import type { RoleSummary } from '../../api/admin'
import { useDeleteRole, useRole } from '../../hooks/useAdmin'
import { areaLabel, opLabel, roleName } from '../../lib/admin-model'
import { statusOf } from '../../lib/errors'
import { formatCount } from '../../lib/format'
import { Avatar, Button, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { SidePanel } from '../spaces/SidePanel'

interface RolePanelProps {
  workspaceId: string
  /** The row that was opened: the panel has a title before the detail arrives. */
  role: RoleSummary
  onEdit: () => void
  onClose: () => void
}

const FACES = 3

/** "Lê Thị Hoa, Nguyễn Thu Lan và 12 người khác". */
function holdersLine(names: string[], total: number): string {
  if (total === 0) return 'Chưa có ai giữ vai trò này.'
  const shown = names.slice(0, FACES - 1)
  const more = total - shown.length
  if (more <= 0) return shown.join(', ')
  return `${shown.join(', ')} và ${more} người khác`
}

/**
 * One role (design/mockups/admin.html §4): who holds it and what it permits, by
 * area, in words ("Xem", "Duyệt"), never by code. A role the administrator made
 * can have its permissions edited or be deleted; the two built-in roles show a
 * lock and cannot. Mount it keyed by the role, so a dialog never carries over.
 */
export function RolePanel({ workspaceId, role, onEdit, onClose }: RolePanelProps) {
  const detail = useRole(workspaceId, role.id)
  const remove = useDeleteRole(workspaceId)
  const [confirming, setConfirming] = useState(false)

  const custom = role.kind === 'custom'
  const name = roleName(role)
  const d = detail.data
  const count = d?.role.member_count ?? role.member_count

  return (
    <SidePanel
      label="Vai trò"
      title={name}
      sub={`${custom ? 'Vai trò tuỳ chỉnh' : 'Vai trò hệ thống'} · ${formatCount(count)} thành viên`}
      closeLabel="vai trò"
      onClose={onClose}
      footer={
        custom && (
          <div className="flex items-center gap-2 px-4 pt-2 pb-4">
            <Button variant="ghost" size="sm" className="text-danger" onClick={() => setConfirming(true)}>
              <Trash2 size={16} strokeWidth={1.75} aria-hidden="true" />
              Xoá vai trò
            </Button>
          </div>
        )
      }
    >
      {detail.isPending ? (
        <div className="grid gap-4 px-3 pt-1 pb-3" aria-busy="true" aria-label="Đang tải vai trò">
          <div className="skeleton h-8 w-40 rounded-sm" />
          <div className="skeleton h-20 rounded-sm" />
          <div className="skeleton h-20 rounded-sm" />
        </div>
      ) : detail.isError ? (
        <div role="alert" className="grid justify-items-start gap-2 px-3 pt-1 pb-3 text-sm text-ink-muted">
          <span className="flex items-start gap-2">
            <CircleAlert size={16} strokeWidth={1.75} className="shrink-0 mt-0.5" aria-hidden="true" />
            {statusOf(detail.error) === 403
              ? 'Bạn không có quyền xem quyền của vai trò này.'
              : 'Không tải được vai trò. Kiểm tra kết nối rồi thử lại.'}
          </span>
          {statusOf(detail.error) !== 403 && <Button variant="soft" size="sm" onClick={() => void detail.refetch()}>Thử lại</Button>}
        </div>
      ) : (
        d && (
          <div className="grid gap-5 px-3 pt-1 pb-3 min-w-0">
            <section className="grid gap-2 min-w-0">
              <h3 className="m-0 text-label text-ink-muted">Thành viên</h3>
              <div className="flex items-start gap-2.5">
                <span className="flex items-center shrink-0">
                  {d.members.slice(0, FACES).map((p, i) => (
                    <Avatar key={p.ngac_node_id} name={p.display_name} hueKey={p.user_id} src={p.avatar_url} size={24} className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`} />
                  ))}
                </span>
                <span className="text-sm leading-snug">{holdersLine(d.members.map((p) => p.display_name), count)}</span>
              </div>
            </section>

            <section className="grid gap-2.5 min-w-0">
              <div className="flex items-center gap-2">
                <h3 className="m-0 text-label text-ink-muted">Được phép</h3>
                {custom && (
                  <Button variant="soft" size="sm" className="ml-auto" onClick={onEdit}>Sửa quyền</Button>
                )}
              </div>
              {d.permissions.length === 0 ? (
                <p className="m-0 text-sm text-ink-muted">
                  {custom ? 'Vai trò này chưa cho quyền nào. Chọn Sửa quyền để thêm.' : 'Vai trò này chưa có quyền nào.'}
                </p>
              ) : (
                <ul aria-label="Quyền theo vùng" className="grid gap-2 m-0 p-0 list-none">
                  {d.permissions.map((g) => (
                    <li key={g.area} className="grid gap-1.5 p-3 rounded-surface bg-sunk">
                      <span className="text-sm font-semibold">{areaLabel(g.area)}</span>
                      <ul aria-label={`Quyền trên ${areaLabel(g.area)}`} className="flex flex-wrap gap-1.5 m-0 p-0 list-none">
                        {g.operations.map((op) => (
                          <li key={op} className="inline-flex items-center h-5.5 px-2 rounded-full bg-accent-wash text-xs font-semibold text-ink">
                            {opLabel(op)}
                          </li>
                        ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              )}
              {!custom && (
                <p className="m-0 flex items-start gap-2 text-sm text-ink-muted">
                  <Lock size={16} strokeWidth={1.75} className="shrink-0 mt-0.5" aria-hidden="true" />
                  Vai trò hệ thống không sửa được quyền.
                </p>
              )}
            </section>
          </div>
        )
      )}

      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        title={`Xoá vai trò ${name}?`}
        description={`${formatCount(count)} người đang giữ vai trò này sẽ mất những quyền nó cho. Việc này không hoàn tác được.`}
        confirmLabel="Xoá vai trò"
        confirmVariant="danger"
        loading={remove.isPending}
        onConfirm={() =>
          remove.mutate(role.id, {
            onSuccess: () => {
              setConfirming(false)
              toast(`Đã xoá vai trò ${name}`)
              onClose()
            },
            onError: () => setConfirming(false),
          })
        }
      />
    </SidePanel>
  )
}
