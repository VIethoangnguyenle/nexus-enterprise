import type { ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Lock } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMemberDirectory } from '../../hooks/useAdmin'
import { statusOf } from '../../lib/errors'
import { workspaceDisplayName } from '../../lib/workspace'
import { Heading, TabBar, type TabItem } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'

export type AdminSection = 'overview' | 'users' | 'roles'

const SECTIONS: { id: AdminSection; label: string; to: '/admin' | '/admin/users' | '/admin/roles' }[] = [
  { id: 'overview', label: 'Tổng quan', to: '/admin' },
  { id: 'users', label: 'Người dùng', to: '/admin/users' },
  { id: 'roles', label: 'Vai trò', to: '/admin/roles' },
]

const TABS: TabItem[] = SECTIONS.map(({ id, label }) => ({ id, label }))

interface AdminFrameProps {
  section: AdminSection
  /** The line under the title: "64 thành viên". Hidden until it is known. */
  subtitle?: string
  /** The section's one primary action, at the right of the title. */
  action?: ReactNode
  children: ReactNode
}

/**
 * The shell of the three Quản trị screens (design/mockups/admin.html): title,
 * the workspace it administers, the section's action and the tabs. Quản trị is
 * a region with a condition, so the frame also asks the people list once: a 403
 * means this person may not manage the workspace, and every tab says so, in
 * words, instead of showing a broken table.
 */
export function AdminFrame({ section, subtitle, action, children }: AdminFrameProps) {
  const navigate = useNavigate()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = useMemberDirectory(wsId)
  const workspace = workspaceDisplayName(workspaceName)
  const forbidden = statusOf(people.error) === 403

  const go = (id: string) => {
    const to = SECTIONS.find((s) => s.id === id)?.to
    // Only the workspace follows to another tab; what was open does not.
    if (to) void navigate({ to, search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })
  }

  return (
    <section className="relative flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Quản trị">
      <header className="flex items-center gap-4 px-5 pt-4 pb-1 min-w-0">
        <div className="grid gap-0.5 min-w-0">
          <Heading as="h1" look="page" className="truncate">Quản trị</Heading>
          <span className="text-sm text-ink-muted truncate">{subtitle ? `${workspace} · ${subtitle}` : workspace}</span>
        </div>
        {!forbidden && action && <div className="ml-auto flex items-center gap-2 shrink-0">{action}</div>}
      </header>

      <div className="px-5 overflow-x-auto">
        <TabBar className="w-max" label="Quản trị" idPrefix="admin" value={section} tabs={TABS} onChange={go} />
      </div>

      <div
        role="tabpanel"
        id={`admin-panel-${section}`}
        aria-labelledby={`admin-tab-${section}`}
        className="relative flex-1 flex min-h-0 min-w-0"
      >
        {forbidden ? (
          <div className="flex-1 grid content-center">
            <EmptyState
              icon={<Lock size={24} strokeWidth={1.75} />}
              text={`Bạn chưa có quyền quản trị ${workspace}. Cần quyền Quản lý. Nhờ chủ sở hữu của workspace cấp cho bạn.`}
            />
          </div>
        ) : (
          children
        )}
      </div>
    </section>
  )
}
