import { Link } from '@tanstack/react-router'
import { CircleAlert, Lock } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import { useWorkspaceDetails } from '../../hooks/useWorkspaces'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { LeaveWorkspace } from './LeaveWorkspace'
import { WorkspaceDetailsForm, WorkspaceTypeCard } from './WorkspaceInfo'

/**
 * Cài đặt → Workspace (mockup §6): the workspace's name and description. A
 * person without Quản lý sees the same fields as plain text with the reason; the
 * server decides who may change them. Members, roles and the rest live in Quản
 * trị, which managers are pointed to.
 */
export function WorkspaceTab() {
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const q = useWorkspaceDetails(wsId)

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

  if (!q.data.can_manage) {
    return (
      <div className="grid gap-5 max-w-130">
        <ReadOnly label="Tên workspace" value={workspaceDisplayName(q.data.name ?? '')} />
        <ReadOnly label="Mô tả" value={q.data.description || 'Chưa có mô tả.'} />
        <WorkspaceTypeCard details={q.data} />
        <p className="m-0 flex items-center gap-2 text-sm text-ink-muted">
          <Lock size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
          Chỉ người có quyền Quản lý mới đổi được tên và mô tả. Nhờ quản trị viên của {workspaceDisplayName(workspaceName)}.
        </p>
        <LeaveWorkspace />
      </div>
    )
  }

  return (
    <div className="grid gap-5 max-w-130">
      <WorkspaceDetailsForm wsId={wsId} details={q.data} label="Workspace" />
      <WorkspaceTypeCard details={q.data} />
      <AdminPointer />
      <LeaveWorkspace />
    </div>
  )
}

function ReadOnly({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1.5">
      <span className="text-sm font-semibold text-ink">{label}</span>
      <div className="flex items-center gap-2 min-h-10 px-3 py-2 rounded-md bg-sunk text-16 text-ink-subtle whitespace-pre-wrap break-words">
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
