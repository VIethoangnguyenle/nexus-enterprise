import type { ReactNode } from 'react'
import { useBottomBarLayout } from '../../hooks/usePhone'
import type { SettingsSection } from '../settings/settings-nav'
import { ModuleMenu } from './ModuleMenu'

/** The settings page behind each module's ⋯; Danh bạ has none (DESIGN.md §6 "Cài đặt"). */
const SETTINGS_OF: Partial<Record<string, SettingsSection>> = {
  messaging: 'chat', calendar: 'calendar', drive: 'drive', approval: 'approval', assets: 'assets',
}

export interface ModuleHeaderProps {
  /** Nav-destination key (e.g. 'drive', 'approval'). */
  moduleId: string
  title: string
  icon?: ReactNode
  /** Right-side action buttons. On mobile: shown in the compact header, before the ⋯. */
  actions?: ReactNode
  /** The module's shortcuts (MenuItem rows) in its ⋯ menu, after Cài đặt <phân hệ>. */
  menu?: ReactNode
  /** ScrollTabs or other sub-header content, rendered below the title row on mobile only. */
  children?: ReactNode
}

/**
 * Compact module header for phones (< 1024px, DESIGN.md §5 "Module header pattern").
 * On desktop (≥ 1024px) it renders nothing — the existing page headers stay as-is.
 *
 * Layout on mobile:
 * ┌──────────────────────────────────┐
 * │ [icon] Title         [actions] [⋯]│  ← title row
 * │ ScrollTabs (optional)             │  ← children
 * └──────────────────────────────────┘
 *
 * The ⋯ opens a menu whose first row is Cài đặt <phân hệ>; Danh bạ has no settings page and so no ⋯
 * unless it brings rows of its own.
 */
export function ModuleHeader({ moduleId, title, icon, actions, menu, children }: ModuleHeaderProps) {
  const isMobile = useBottomBarLayout()
  if (!isMobile) return null
  const section = SETTINGS_OF[moduleId]
  const hasMenu = !!section || !!menu

  return (
    <header className="border-b border-line bg-base">
      <div className="flex items-center justify-between px-4 py-3">
        <div className="flex items-center gap-2 min-w-0">
          {icon && <span className="shrink-0">{icon}</span>}
          <h1 className="text-lg font-semibold text-ink truncate">{title}</h1>
        </div>
        {(actions || hasMenu) && (
          <div className="flex items-center gap-2 shrink-0">
            {actions}
            {hasMenu && <ModuleMenu section={section} label={title}>{menu}</ModuleMenu>}
          </div>
        )}
      </div>
      {children}
    </header>
  )
}
