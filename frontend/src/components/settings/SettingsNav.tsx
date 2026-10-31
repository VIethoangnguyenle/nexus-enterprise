import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Bell, Building2, Calendar, ClipboardCheck, FileText, MessageSquare, Package, Palette, Settings, Smartphone, User,
} from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { appVersion } from '../../lib/app-version'
import { workspaceDisplayName } from '../../lib/workspace'
import type { SettingsSection } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const

interface Item { section: SettingsSection; label: string; icon: ReactNode }

const ACCOUNT: Item[] = [
  { section: 'account', label: 'Tài khoản và hồ sơ', icon: <User {...ICON} /> },
  { section: 'notifications', label: 'Thông báo', icon: <Bell {...ICON} /> },
  { section: 'appearance', label: 'Giao diện, ngôn ngữ', icon: <Palette {...ICON} /> },
  { section: 'sessions', label: 'Thiết bị và phiên', icon: <Smartphone {...ICON} /> },
]

/** The workspace's items; `org` marks those a personal workspace does not have. */
const WORKSPACE: Array<Item & { org?: true }> = [
  { section: 'org-profile', label: 'Hồ sơ trong tổ chức', icon: <Building2 {...ICON} />, org: true },
  { section: 'chat', label: 'Chat', icon: <MessageSquare {...ICON} /> },
  { section: 'calendar', label: 'Lịch', icon: <Calendar {...ICON} /> },
  { section: 'drive', label: 'Tài liệu', icon: <FileText {...ICON} /> },
  { section: 'approval', label: 'Phê duyệt', icon: <ClipboardCheck {...ICON} />, org: true },
  { section: 'assets', label: 'Tài sản', icon: <Package {...ICON} />, org: true },
  { section: 'workspace', label: 'Cài đặt workspace', icon: <Settings {...ICON} /> },
]

const keepWorkspace = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})

/**
 * The section list of desktop Cài đặt (DESIGN.md §6 "Cài đặt trên máy tính", mockup settings-desktop.html): the two
 * groups of Tôi, each item a real link to `/settings/<section>` so reload, Back and a shared address land on it, and
 * the build at the bottom. `current` is passed in rather than read from the address because `/settings` itself shows
 * Tài khoản và hồ sơ.
 */
export function SettingsNav({ current }: { current: SettingsSection }) {
  const { workspaceName } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const workspaceItems = isPersonal ? WORKSPACE.filter((i) => !i.org) : WORKSPACE

  const item = ({ section, label, icon }: Item) => {
    const here = section === current
    return (
      <Link
        key={section}
        to="/settings/$section"
        params={{ section }}
        search={keepWorkspace}
        aria-current={here ? 'page' : undefined}
        className={`flex items-center gap-2.5 min-h-9 px-2.5 rounded-surface transition-colors duration-quick
          ${here ? 'bg-accent-wash text-accent font-semibold' : 'text-ink font-medium hover:bg-hover'}`}
      >
        <span aria-hidden="true" className={`shrink-0 ${here ? 'text-accent' : 'text-ink-muted'}`}>{icon}</span>
        {label}
      </Link>
    )
  }

  const heading = (text: string, first = false) => (
    <div className={`px-2.5 pb-1.5 text-xs font-semibold text-ink-muted ${first ? 'pt-2' : 'pt-4'}`}>{text}</div>
  )

  return (
    <nav aria-label="Mục cài đặt" className="grid gap-0.5 content-start min-h-0 overflow-y-auto px-2.5 pt-2 pb-4 border-r border-line">
      {heading('Tài khoản', true)}
      {ACCOUNT.map(item)}
      {heading(isPersonal ? 'Trong workspace này' : `Trong ${workspaceDisplayName(workspaceName)}`)}
      {workspaceItems.map(item)}
      <p className="m-0 mt-4 px-2.5 text-xs text-ink-muted select-text">{`Giới thiệu · ${appVersion()}`}</p>
    </nav>
  )
}
