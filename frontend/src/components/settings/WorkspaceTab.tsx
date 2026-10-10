import { useEffect, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { CircleAlert, Lock } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import { useUpdateWorkspaceDetails, useWorkspaceDetails } from '../../hooks/useWorkspaces'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, Textarea, TextField, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { LeaveWorkspace } from './LeaveWorkspace'

const NAME_MAX = 100
const DESCRIPTION_MAX = 500

/** Name and description are the only parts of a workspace that this screen can change. */
interface Form { name: string; description: string }

/**
 * Cài đặt → Workspace (mockup §6): the workspace's name and description. A
 * person without Quản lý sees the same fields as plain text with the reason; the
 * server decides who may change them. Members, roles and the rest live in Quản
 * trị, which managers are pointed to.
 */
export function WorkspaceTab() {
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const q = useWorkspaceDetails(wsId)
  const save = useUpdateWorkspaceDetails(wsId)

  const initial: Form = { name: q.data?.name ?? '', description: q.data?.description ?? '' }
  const [form, setForm] = useState<Form>(initial)
  const [nameError, setNameError] = useState<string | undefined>()
  const nameRef = useRef<HTMLInputElement>(null)

  // The saved values arrive (or change after a save): a form nobody has edited follows them.
  const seeded = useRef<Form | null>(null)
  useEffect(() => {
    if (!q.data) return
    const pristine = !seeded.current || (form.name === seeded.current.name && form.description === seeded.current.description)
    seeded.current = initial
    if (pristine) setForm(initial)
     
  }, [q.data])

  if (q.isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được thông tin workspace. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>}
      />
    )
  }
  if (!q.data) {
    return (
      <div aria-busy="true" className="grid gap-5 max-w-130">
        <span className="skeleton h-10 rounded-md" />
        <span className="skeleton h-24 rounded-md" />
      </div>
    )
  }

  const canManage = q.data.can_manage
  const name = form.name.trim()
  const description = form.description.trim()
  const changed = name !== initial.name || description !== initial.description

  const checkName = (value: string) => {
    const v = value.trim()
    if (!v) return 'Nhập tên cho workspace.'
    if (Array.from(v).length > NAME_MAX) return `Tên dài tối đa ${NAME_MAX} ký tự.`
    return undefined
  }

  const submit = async () => {
    const err = checkName(form.name)
    setNameError(err)
    if (err) return nameRef.current?.focus()
    if (!changed) return
    const body: { name?: string; description?: string } = {}
    if (name !== initial.name) body.name = name
    if (description !== initial.description) body.description = description
    try {
      await save.mutateAsync(body)
    } catch {
      return // told by the shared handler; the form keeps what was typed
    }
    toast('Đã lưu workspace')
  }

  if (!canManage) {
    return (
      <div className="grid gap-5 max-w-130">
        <ReadOnly label="Tên workspace" value={workspaceDisplayName(initial.name)} />
        <ReadOnly label="Mô tả" value={initial.description || 'Chưa có mô tả.'} />
        <p className="m-0 flex items-center gap-2 text-sm text-ink-muted">
          <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
          Chỉ người có quyền Quản lý mới đổi được tên và mô tả. Nhờ quản trị viên của {workspaceDisplayName(workspaceName)}.
        </p>
        <LeaveWorkspace />
      </div>
    )
  }

  return (
    <form
      aria-label="Workspace"
      noValidate
      onSubmit={(e) => { e.preventDefault(); void submit() }}
      className="grid gap-5 max-w-130"
    >
      <TextField
        ref={nameRef}
        label="Tên workspace"
        value={form.name}
        maxLength={NAME_MAX + 20}
        error={nameError}
        onChange={(e) => {
          setForm((f) => ({ ...f, name: e.target.value }))
          if (nameError) setNameError(checkName(e.target.value))
        }}
        onBlur={() => setNameError(checkName(form.name))}
      />
      <div className="grid gap-1.5">
        <label htmlFor="workspace-description" className="text-sm font-semibold text-ink">
          Mô tả <span className="font-normal text-ink-muted">(không bắt buộc)</span>
        </label>
        <Textarea
          id="workspace-description"
          rows={3}
          maxLength={DESCRIPTION_MAX}
          value={form.description}
          onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
        />
      </div>

      <div className="flex items-center gap-2">
        <Button type="submit" loading={save.isPending} disabled={!changed}>Lưu thay đổi</Button>
        <Button
          type="button"
          variant="ghost"
          disabled={!changed || save.isPending}
          onClick={() => { setForm(initial); setNameError(undefined) }}
        >
          Huỷ
        </Button>
      </div>

      <AdminPointer />
      <LeaveWorkspace />
    </form>
  )
}

function ReadOnly({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1.5">
      <span className="text-sm font-semibold text-ink">{label}</span>
      <div className="flex items-center gap-2 min-h-10 px-3 py-2 rounded-md bg-sunk text-base text-ink-subtle whitespace-pre-wrap break-words">
        {value}
      </div>
    </div>
  )
}

/** For managers only: how big the workspace is, and where to change who is in it. */
function AdminPointer() {
  const { workspaceId: wsId } = useActiveWorkspace()
  const members = useMemberDirectory(wsId)
  const roles = useRoles(wsId)
  const parts = [
    members.data ? `${members.data.length} thành viên` : '',
    roles.data ? `${roles.data.custom.length + roles.data.system.length} vai trò` : '',
  ].filter(Boolean)
  return (
    <div className="flex items-center gap-4 p-4 rounded-surface bg-raised">
      <div className="grid flex-1 min-w-0">
        <span className="font-semibold text-ink">{parts.join(', ') || 'Thành viên và vai trò'}</span>
        <span className="text-sm text-ink-muted">Mời người, đổi vai trò và phòng ban ở Quản trị.</span>
      </div>
      <Link
        to="/admin"
        search={(prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})}
        className="inline-flex items-center h-8 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink no-underline
          hover:bg-line focus-ring transition-colors duration-quick"
      >
        Mở Quản trị
      </Link>
    </div>
  )
}
