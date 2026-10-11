import { useEffect, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  AlertCircle, Building2, CircleAlert, ClipboardCheck, FileText, Key, Lock, Package, User,
} from 'lucide-react'
import { ApiError } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import {
  useConvertToOrganization, useUpdateWorkspaceDetails, useWorkspaceDetails,
} from '../../hooks/useWorkspaces'
import { formatDateTime, formatRelative } from '../../lib/format'
import { workspaceDisplayName } from '../../lib/workspace'
import type { WorkspaceDetails } from '../../api/workspaces'
import { Avatar, Button, PersonChip, Textarea, TextField, toast } from '../primitives'
import { Dialog } from '../composites/Dialog'
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
  const [convertOpen, setConvertOpen] = useState(false)

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
        <WorkspaceTypeCard
          details={q.data}
          onOpenConvert={() => setConvertOpen(true)}
        />
        <p className="m-0 flex items-center gap-2 text-sm text-ink-muted">
          <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
          Chỉ người có quyền Quản lý mới đổi được tên và mô tả. Nhờ quản trị viên của {workspaceDisplayName(workspaceName)}.
        </p>
        <LeaveWorkspace />
        <ConvertDialog
          open={convertOpen}
          onClose={() => setConvertOpen(false)}
          workspaceName={workspaceName}
          wsId={wsId}
          onSuccess={() => void q.refetch()}
        />
      </div>
    )
  }

  return (
    <>
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

        <WorkspaceTypeCard
          details={q.data}
          onOpenConvert={() => setConvertOpen(true)}
        />

        <AdminPointer />
        <LeaveWorkspace />
      </form>

      <ConvertDialog
        open={convertOpen}
        onClose={() => setConvertOpen(false)}
        workspaceName={workspaceName}
        wsId={wsId}
        onSuccess={() => void q.refetch()}
      />
    </>
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

function WorkspaceTypeCard({
  details,
  onOpenConvert,
}: {
  details: WorkspaceDetails
  onOpenConvert: () => void
}) {
  const isPersonal = details.type === 'personal'
  const isOwner = !!details.is_owner
  // The server joins the owners' names with ", ".
  const owners = (details.owner_name ?? '').split(', ').map((n) => n.trim()).filter(Boolean)

  // After a conversion the button that opened the dialog is gone; keyboard focus
  // goes to the next step instead of falling to the page.
  const adminLinkRef = useRef<HTMLAnchorElement>(null)
  const wasPersonal = useRef(isPersonal)
  useEffect(() => {
    if (wasPersonal.current && !isPersonal) adminLinkRef.current?.focus()
    wasPersonal.current = isPersonal
  }, [isPersonal])

  if (isPersonal) {
    return (
      <section aria-labelledby="workspace-type-title" className="grid gap-3 p-4 rounded-surface bg-raised">
        <div className="flex items-start gap-3">
          <span
            aria-hidden="true"
            className="grid place-items-center w-10 h-10 rounded-lg bg-sunk text-ink-muted shrink-0"
          >
            <User size={20} strokeWidth={1.75} />
          </span>
          <div className="grid gap-1 min-w-0">
            <h3 id="workspace-type-title" className="font-semibold text-base text-ink m-0">
              Workspace cá nhân
            </h3>
            <span className="text-sm text-ink-muted">
              Có Tin nhắn, Tài liệu và Danh bạ. Không có Tài sản, Phê duyệt và duyệt tài liệu.
            </span>
          </div>
        </div>

        {isOwner ? (
          <div className="pl-13 max-sm:pl-0">
            <Button
              type="button"
              variant="soft"
              onClick={onOpenConvert}
              className="max-sm:w-full max-sm:min-h-11"
            >
              <Building2 size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
              Chuyển thành workspace tổ chức
            </Button>
          </div>
        ) : (
          <p className="m-0 flex items-center gap-2 pl-13 max-sm:pl-0 text-sm text-ink-muted flex-wrap">
            <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
            <span>
              Chỉ chủ sở hữu,{' '}
              <span className="inline-flex flex-wrap items-center gap-x-2 gap-y-1 align-middle">
                {(owners.length ? owners : ['chủ sở hữu']).map((owner) => (
                  <PersonChip key={owner} name={owner} className="font-medium text-ink" />
                ))}
              </span>
              , mới chuyển được workspace này thành workspace tổ chức.
            </span>
          </p>
        )}
      </section>
    )
  }

  // A type this build does not know is not guessed at: the card is left out.
  if (details.type !== 'organization') return null

  return (
    <section aria-labelledby="workspace-type-title" className="grid gap-3 p-4 rounded-surface bg-raised">
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          className="grid place-items-center w-10 h-10 rounded-lg bg-accent-wash text-accent shrink-0"
        >
          <Building2 size={20} strokeWidth={1.75} />
        </span>
        <div className="grid gap-1 min-w-0">
          <h3 id="workspace-type-title" className="font-semibold text-base text-ink m-0">
            Workspace tổ chức
          </h3>
          <span className="text-sm text-ink-muted">
            Có đủ Tin nhắn, Tài liệu, Phê duyệt, Tài sản và Danh bạ.
          </span>
        </div>
      </div>

      {details.converted_by_name && (
        <div className="flex items-center gap-2 pl-13 max-sm:pl-0 text-sm text-ink-muted flex-wrap">
          <Avatar size={20} name={details.converted_by_name} />
          <span>
            <strong className="text-ink font-semibold">{details.converted_by_name}</strong> đã chuyển sang workspace tổ chức
            {details.converted_at && (
              <>
                {' · '}
                <time dateTime={details.converted_at} title={formatDateTime(details.converted_at)}>
                  {formatRelative(details.converted_at) || 'vừa xong'}
                </time>
              </>
            )}
          </span>
        </div>
      )}

      {details.converted_by_name && (
        <div className="pl-13 max-sm:pl-0">
          <Link
            ref={adminLinkRef}
            to="/admin"
            search={(prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})}
            className="inline-flex items-center gap-2 h-8 max-sm:min-h-11 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink no-underline
              hover:bg-line focus-ring transition-colors duration-quick max-sm:w-full max-sm:justify-center"
          >
            <Key size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
            Đặt quyền Phê duyệt và Tài sản
          </Link>
        </div>
      )}
    </section>
  )
}

function ConvertDialog({
  open,
  onClose,
  workspaceName,
  wsId,
  onSuccess,
}: {
  open: boolean
  onClose: () => void
  workspaceName: string
  wsId: string
  onSuccess: () => void
}) {
  const cancelBtnRef = useRef<HTMLButtonElement>(null)
  const convertMutation = useConvertToOrganization(wsId)

  const handleConvert = async () => {
    try {
      await convertMutation.mutateAsync()
      onSuccess()
      onClose()
      toast('Đã chuyển thành workspace tổ chức')
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          toast('Chỉ chủ sở hữu mới chuyển được workspace này.')
          onSuccess()
          return
        }
        if (err.status === 409) {
          toast('Workspace này đã là workspace tổ chức.')
          onSuccess()
          onClose()
          return
        }
      }
      toast('Không chuyển được workspace. Kiểm tra kết nối rồi thử lại.')
    }
  }

  return (
    <Dialog
      open={open}
      onClose={() => {
        if (!convertMutation.isPending) onClose()
      }}
      title="Chuyển thành workspace tổ chức?"
      initialFocusRef={cancelBtnRef}
      role="alertdialog"
      footer={
        <div className="flex justify-end gap-2 max-sm:flex-col-reverse w-full sm:w-auto">
          <Button
            ref={cancelBtnRef}
            type="button"
            variant="ghost"
            disabled={convertMutation.isPending}
            onClick={onClose}
            className="max-sm:w-full min-h-11 sm:min-h-9"
          >
            Huỷ
          </Button>
          <Button
            type="button"
            variant="primary"
            loading={convertMutation.isPending}
            onClick={() => void handleConvert()}
            className="max-sm:w-full min-h-11 sm:min-h-9"
          >
            Chuyển thành tổ chức
          </Button>
        </div>
      }
    >
      <div className="grid gap-4 text-sm text-ink">
        <p className="m-0">
          Mọi người trong <strong>{workspaceDisplayName(workspaceName)}</strong> sẽ thấy thêm:
        </p>
        <ul className="grid gap-2 m-0 p-0 list-none">
          <li className="grid grid-cols-[2rem_minmax(0,1fr)] items-center gap-3 py-1">
            <span
              aria-hidden="true"
              className="grid place-items-center w-8 h-8 rounded-lg bg-warning-wash text-warning shrink-0"
            >
              <Package size={16} strokeWidth={1.75} />
            </span>
            <span className="grid gap-0.5">
              <strong className="font-semibold text-ink">Tài sản</strong>
              <span className="text-ink-muted">Kho tài sản, cấp và thu hồi tài sản.</span>
            </span>
          </li>
          <li className="grid grid-cols-[2rem_minmax(0,1fr)] items-center gap-3 py-1">
            <span
              aria-hidden="true"
              className="grid place-items-center w-8 h-8 rounded-lg bg-warning-wash text-warning shrink-0"
            >
              <ClipboardCheck size={16} strokeWidth={1.75} />
            </span>
            <span className="grid gap-0.5">
              <strong className="font-semibold text-ink">Phê duyệt</strong>
              <span className="text-ink-muted">Tạo đề nghị, chuỗi người duyệt, Duyệt và Từ chối.</span>
            </span>
          </li>
          <li className="grid grid-cols-[2rem_minmax(0,1fr)] items-center gap-3 py-1">
            <span
              aria-hidden="true"
              className="grid place-items-center w-8 h-8 rounded-lg bg-warning-wash text-warning shrink-0"
            >
              <FileText size={16} strokeWidth={1.75} />
            </span>
            <span className="grid gap-0.5">
              <strong className="font-semibold text-ink">Duyệt tài liệu</strong>
              <span className="text-ink-muted">Nút Gửi duyệt trong văn bản.</span>
            </span>
          </li>
        </ul>
        <div className="grid gap-1 text-sm text-ink-muted">
          <p className="m-0">Tin nhắn, tài liệu, thành viên và vai trò giữ nguyên.</p>
          <p className="m-0">Tài sản và đề nghị phê duyệt có từ trước, đang bị ẩn, sẽ hiện lại cho người có quyền xem.</p>
        </div>
        <div className="flex gap-2.5 items-start p-3 rounded-lg bg-warning-wash text-ink text-sm">
          <AlertCircle size={18} className="text-warning shrink-0 mt-0.5" aria-hidden="true" />
          <span>
            <strong>Không chuyển ngược lại được.</strong> Workspace tổ chức không trở về workspace cá nhân.
          </span>
        </div>
      </div>
    </Dialog>
  )
}
