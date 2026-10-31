import { Link, useLocation } from '@tanstack/react-router'
import { Check, Mail, MoreHorizontal, Plus, RefreshCw } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useOpenInvitationCount } from '../../hooks/useInvitations'
import { useWorkspaceSwitcher } from '../../hooks/useSwitchWorkspace'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, IconButton, Pressable, Spinner, Text, spaceLetter } from '../primitives'
import { useSettingsNav } from '../settings/settings-nav'
import { NavSheet } from '../spaces/NavSheet'
import { CountPill } from './CountPill'

/** What kind of workspace this is, in a line: "Workspace tổ chức · 12 thành viên". */
export function useWorkspaceKindLine(): string {
  const { isPersonal } = useWorkspaceType()
  const { currentId, workspaces } = useWorkspaceSwitcher({ landOnChat: true })
  const current = workspaces.find((w) => w.id === currentId)
  if (isPersonal) return 'Workspace cá nhân · chỉ bạn'
  return current?.member_count ? `Workspace tổ chức · ${current.member_count} thành viên` : 'Workspace tổ chức'
}

/**
 * The workspaces the person can enter (DESIGN.md §5, Điều hướng di động), as a sheet: opened by the letter
 * tile at the left of the search pill and by the workspace row of Thêm. Choosing one re-scopes the session
 * (`useWorkspaceSwitcher`) and closes the sheet; Lời mời and Tạo workspace are the last rows. The ⋯ on the
 * open workspace opens Cài đặt workspace. Render it only while open.
 */
export function WorkspaceSwitcherSheet({ onClose }: { onClose: () => void }) {
  const { workspaceName } = useActiveWorkspace()
  const offerCount = useOpenInvitationCount()
  const kind = useWorkspaceKindLine()
  const name = workspaceDisplayName(workspaceName)
  const settings = useSettingsNav()

  return (
    <NavSheet label="Đổi workspace" onClose={onClose}>
      <div className="grid grid-cols-[2.5rem_minmax(0,1fr)_2.75rem] items-center gap-3 pl-3.5 pr-1.5 pt-1 pb-2">
        <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold">
          {spaceLetter(name)}
        </span>
        <span className="grid min-w-0 gap-0.5">
          <span className="font-display text-[0.9375rem] font-bold text-ink truncate">{name}</span>
          <span className="text-xs text-ink-muted truncate">{kind}</span>
        </span>
        <IconButton
          size="lg"
          className="size-11"
          aria-label="Cài đặt workspace"
          onClick={() => { settings.open('workspace'); onClose() }}
        >
          <MoreHorizontal size={20} strokeWidth={1.75} />
        </IconButton>
      </div>
      <WorkspaceList onDone={onClose} offerCount={offerCount} />
    </NavSheet>
  )
}

function WorkspaceList({ onDone, offerCount }: { onDone: () => void; offerCount: number }) {
  const { currentId, workspaces, isLoading, isError, refetch, choose, switchingId } = useWorkspaceSwitcher({ landOnChat: true })
  const busy = switchingId !== undefined
  // Where "Tạo workspace" sends Back to: the screen the person is on now.
  const here = useLocation({ select: (l) => l.href })

  if (isError) {
    return (
      <div className="grid justify-items-center gap-3 px-4 py-6 text-center">
        <Text variant="body" className="block max-w-64">
          Không tải được danh sách workspace. Kiểm tra kết nối rồi thử lại.
        </Text>
        <Button variant="soft" size="sm" onClick={() => void refetch()}>
          <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
          Thử lại
        </Button>
      </div>
    )
  }
  if (isLoading) {
    return (
      <ul className="grid gap-1 m-0 p-0" aria-busy="true">
        {[0, 1].map((i) => (
          <li key={i} data-skeleton className="list-none flex items-center gap-3 min-h-12 px-3.5">
            <div className="skeleton w-7 h-7 rounded-lg" />
            <div className="skeleton h-3.5 w-3/5 rounded-md" />
          </li>
        ))}
      </ul>
    )
  }
  return (
    <div className="grid gap-0.5">
      <ul role="listbox" aria-label="Workspace của bạn" className="grid gap-0.5 m-0 p-0">
        {workspaces.map((w) => {
          const name = workspaceDisplayName(w.name)
          const isCurrent = w.id === currentId
          return (
            <li key={w.id} role="presentation" className="list-none">
              <Pressable
                role="option"
                aria-selected={isCurrent}
                disabled={busy}
                onClick={() => choose(w.id, onDone)}
                className="grid grid-cols-[1.75rem_minmax(0,1fr)_1.25rem] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
                  text-[0.9375rem] font-medium text-ink hover:bg-hover transition-colors duration-quick"
              >
                <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-lg bg-accent text-on-accent font-display text-sm font-bold">
                  {spaceLetter(name)}
                </span>
                <span className="truncate">{name}</span>
                {switchingId === w.id
                  ? <Spinner size="sm" />
                  : isCurrent && <Check size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />}
              </Pressable>
            </li>
          )
        })}
      </ul>
      {offerCount > 0 && (
        <Link
          to="/workspace-select"
          onClick={onDone}
          className="grid grid-cols-[1.75rem_minmax(0,1fr)_auto] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
            text-[0.9375rem] font-medium text-ink no-underline hover:bg-hover transition-colors duration-quick focus-ring"
        >
          <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-lg bg-sunk text-ink-muted">
            <Mail size={16} strokeWidth={1.75} />
          </span>
          <span>Lời mời</span>
          <CountPill n={offerCount} />
          <span className="sr-only">, {offerCount} lời mời đang chờ</span>
        </Link>
      )}
      <Link
        to="/onboarding"
        search={{ from: here }}
        onClick={onDone}
        className="grid grid-cols-[1.75rem_minmax(0,1fr)] items-center gap-3 w-full min-h-12 px-3.5 py-1.5 rounded-full
          text-[0.9375rem] font-medium text-accent no-underline hover:bg-hover transition-colors duration-quick focus-ring"
      >
        <span aria-hidden="true" className="grid place-items-center w-7 h-7"><Plus size={20} strokeWidth={1.75} /></span>
        <span>Tạo workspace</span>
      </Link>
    </div>
  )
}
