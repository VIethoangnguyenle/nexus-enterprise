import { Navigate, useLocation, useSearch } from '@tanstack/react-router'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { sectionOfTab, tabOf, type SettingsSearch } from '../../lib/settings-search'
import { AccountScreen } from './AccountScreen'
import { AppearanceScreen } from './AppearanceScreen'
import { MeScreen } from './MeScreen'
import { NotificationsScreen } from './NotificationsScreen'
import { SettingsShell } from './SettingsShell'
import { WorkspaceSettingsScreen } from './WorkspaceSettingsScreen'

const keepWorkspace = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})

/**
 * `/settings`. On a wide screen (DESIGN.md §6 "Cài đặt trên máy tính"): the section list with Tài khoản và hồ sơ
 * open; an old tab address (`?tab=giao-dien`, or a tab it does not know) moves on to the section that holds its
 * options, replacing itself so Back does not bounce. Below 1024 there is no list: `/settings` is Tôi (§6 "Cài đặt"), and an old tab address opens
 * the page that holds the same options.
 */
export function SettingsScreen() {
  const phone = useBottomBarLayout()
  const search = useSearch({ strict: false }) as SettingsSearch
  const tab = tabOf(search)
  // The address as written, before validation drops `tab=ho-so` or a tab this build does not know.
  const namesTab = useLocation({ select: (l) => 'tab' in (l.search as Record<string, unknown>) })
  if (phone) {
    switch (tab) {
      case 'thong-bao': return <NotificationsScreen />
      case 'giao-dien': return <AppearanceScreen />
      case 'workspace': return <WorkspaceSettingsScreen />
      default: return <MeScreen />
    }
  }
  if (namesTab) {
    return <Navigate to="/settings/$section" params={{ section: sectionOfTab(tab) }} search={keepWorkspace} replace />
  }
  return <SettingsShell section="account"><AccountScreen /></SettingsShell>
}
