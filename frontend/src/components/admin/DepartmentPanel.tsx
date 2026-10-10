import { useMemo, useState } from 'react'
import { Pencil, Trash2 } from 'lucide-react'
import type { Department, Member } from '../../api/admin'
import { useDeleteDepartment, useMoveDepartment, useUpdateDepartment, useUpdateMemberDepartment } from '../../hooks/useAdmin'
import { deptWithDescendants, personFromMember } from '../../lib/admin-model'
import { formatCount } from '../../lib/format'
import { Button, PeoplePicker, PersonChip, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { NameDialog } from '../drive/NameDialog'
import { SidePanel } from '../spaces/SidePanel'
import { DepartmentPicker } from './DepartmentPicker'

/** People listed by name before "Xem cả N người". */
const SHOWN = 8

interface DepartmentPanelProps {
  workspaceId: string
  dept: Department
  departments: Department[]
  members: Member[]
  onClose: () => void
}

/**
 * One department (design/mockups/admin.html §1): where it sits, who is in it, and
 * what can be done to it. Mount it keyed by the department, so a dialog or a
 * half-typed name never carries over to the next one. Where it sits is chosen
 * from the tree by name; a department cannot be moved under itself.
 */
export function DepartmentPanel({ workspaceId, dept, departments, members, onClose }: DepartmentPanelProps) {
  const rename = useUpdateDepartment(workspaceId)
  const remove = useDeleteDepartment(workspaceId)
  const move = useMoveDepartment(workspaceId)
  const assign = useUpdateMemberDepartment(workspaceId)
  const [renaming, setRenaming] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [all, setAll] = useState(false)

  const inside = useMemo(() => members.filter((m) => m.department?.id === dept.id), [members, dept.id])
  const shown = all ? inside : inside.slice(0, SHOWN)
  const candidates = useMemo(() => members.map(personFromMember), [members])
  const presentIds = useMemo(() => new Set(inside.map((m) => m.user_id)), [inside])
  const blocked = useMemo(() => deptWithDescendants(departments, dept.id), [departments, dept.id])

  const parentName = (id: string) => departments.find((d) => d.id === id)?.name

  return (
    <SidePanel
      label="Phòng ban"
      title={dept.name}
      sub={`${formatCount(dept.member_count)} thành viên`}
      closeLabel="phòng ban"
      onClose={onClose}
      footer={
        <div className="flex items-center gap-2 px-4 pt-2 pb-4">
          <Button variant="ghost" size="sm" className="text-danger" onClick={() => setConfirming(true)}>
            <Trash2 size={16} strokeWidth={1.75} aria-hidden="true" />
            Xoá phòng
          </Button>
          <Button variant="soft" size="sm" className="ml-auto" onClick={() => setRenaming(true)}>
            <Pencil size={16} strokeWidth={1.75} aria-hidden="true" />
            Đổi tên
          </Button>
        </div>
      }
    >
      <div className="grid gap-5 px-3 pt-1 pb-3 min-w-0">
        <section className="grid gap-2 min-w-0">
          <h3 className="m-0 text-label text-ink-muted">Thuộc phòng</h3>
          <DepartmentPicker
            label="Thuộc phòng"
            departments={departments}
            value={dept.parent_id}
            exclude={blocked}
            noneLabel="Không có (phòng gốc)"
            disabled={move.isPending}
            onChange={(id) => {
              if (id === dept.parent_id) return
              move.mutate(
                { deptId: dept.id, newParentId: id },
                { onSuccess: () => toast(id ? `Đã chuyển ${dept.name} vào ${parentName(id) ?? 'phòng mới'}` : `Đã chuyển ${dept.name} lên cấp gốc`) },
              )
            }}
          />
        </section>

        <section className="grid gap-2.5 min-w-0">
          <h3 className="m-0 text-label text-ink-muted">Thành viên</h3>
          <PeoplePicker
            label="Thêm thành viên vào phòng"
            people={candidates}
            value={[]}
            exclude={presentIds}
            max={1}
            placeholder="Thêm người vào phòng"
            onChange={(next) => {
              const p = next[0]
              if (!p) return
              assign.mutate(
                { nodeId: p.nodeId, departmentId: dept.id },
                { onSuccess: () => toast(`Đã thêm ${p.name} vào ${dept.name}`) },
              )
            }}
          />
          {inside.length === 0 ? (
            <p className="m-0 text-sm text-ink-muted">Chưa có ai trong phòng này.</p>
          ) : (
            <ul aria-label="Thành viên của phòng" className="grid gap-2 m-0 p-0 list-none">
              {shown.map((m) => (
                <li key={m.ngac_node_id}>
                  <PersonChip name={m.display_name} hueKey={m.user_id} avatarUrl={m.avatar_url} role={m.title} className="max-w-full" />
                </li>
              ))}
            </ul>
          )}
          {inside.length > SHOWN && (
            <Button variant="link" size="link" className="justify-self-start" onClick={() => setAll((v) => !v)}>
              {all ? 'Thu gọn' : `Xem cả ${inside.length} người`}
            </Button>
          )}
        </section>
      </div>

      <NameDialog
        open={renaming}
        onClose={() => setRenaming(false)}
        title="Đổi tên phòng ban"
        submitLabel="Lưu"
        emptyMessage="Đặt tên cho phòng ban."
        initialName={dept.name}
        pending={rename.isPending}
        onSubmit={(name) =>
          rename.mutate(
            { deptId: dept.id, name },
            { onSuccess: () => { setRenaming(false); toast('Đã đổi tên phòng ban') } },
          )
        }
      />
      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        title={`Xoá phòng ${dept.name}?`}
        description="Các phòng con và thành viên sẽ chuyển lên phòng cấp trên. Việc này không hoàn tác được."
        confirmLabel="Xoá phòng"
        confirmVariant="danger"
        loading={remove.isPending}
        onConfirm={() =>
          remove.mutate(dept.id, {
            onSuccess: () => {
              setConfirming(false)
              toast(`Đã xoá phòng ${dept.name}`)
              onClose()
            },
            onError: () => setConfirming(false),
          })
        }
      />
    </SidePanel>
  )
}
