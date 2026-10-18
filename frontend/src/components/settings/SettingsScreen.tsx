import { useNavigate, useSearch } from '@tanstack/react-router'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { appVersionLabel } from '../../lib/app-version'
import { SETTINGS_TABS, tabOf, tabSearch, type SettingsSearch, type SettingsTab } from '../../lib/settings-search'
import { Heading, ScrollFade, TabBar, type TabItem } from '../primitives'
import { AppearanceTab } from './AppearanceTab'
import { ProfileTab } from './ProfileTab'
import { WorkspaceTab } from './WorkspaceTab'
import { NotificationSettings } from './NotificationSettings'
import { AppearanceScreen } from './AppearanceScreen'
import { MeScreen } from './MeScreen'
import { NotificationsScreen } from './NotificationsScreen'
import { WorkspaceSettingsScreen } from './WorkspaceSettingsScreen'

const TABS: TabItem[] = [
  { id: 'ho-so', label: 'Hồ sơ' },
  { id: 'workspace', label: 'Workspace' },
  { id: 'giao-dien', label: 'Giao diện' },
  { id: 'thong-bao', label: 'Thông báo' },
]

/**
 * Cài đặt. On a wide screen (design/mockups/contacts-documents-settings.html
 * §4–6): tabs, the open one kept in the URL (`?tab=giao-dien`) so reload, Back
 * and a link land on it. Below 1024 there are no tabs: `/settings` is Tôi
 * (DESIGN.md §6 "Cài đặt"), and an old link to a tab opens the page that holds
 * the same options now.
 */
export function SettingsScreen() {
  const phone = useBottomBarLayout()
  const search = useSearch({ strict: false }) as SettingsSearch
  if (phone) {
    switch (tabOf(search)) {
      case 'thong-bao': return <NotificationsScreen />
      case 'giao-dien': return <AppearanceScreen />
      case 'workspace': return <WorkspaceSettingsScreen />
      default: return <MeScreen />
    }
  }
  return <SettingsTabs search={search} />
}

function SettingsTabs({ search }: { search: SettingsSearch }) {
  const navigate = useNavigate({ from: '/settings' })
  const tab = tabOf(search)

  const go = (id: string) => {
    if (!(SETTINGS_TABS as readonly string[]).includes(id)) return
    void navigate({ search: (prev: SettingsSearch) => tabSearch(prev, id as SettingsTab) })
  }

  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Cài đặt">
      <header className="px-5 pt-4 pb-1">
        <Heading as="h1" look="page">Cài đặt</Heading>
      </header>
      <ScrollFade className="px-5">
        <TabBar className="w-max" label="Cài đặt" idPrefix="settings" value={tab} tabs={TABS} onChange={go} />
      </ScrollFade>
      <div
        role="tabpanel"
        id={`settings-panel-${tab}`}
        aria-labelledby={`settings-tab-${tab}`}
        className="flex-1 min-h-0 overflow-y-auto px-5 py-5"
      >
        {tab === 'ho-so' && <ProfileTab />}
        {tab === 'workspace' && <WorkspaceTab />}
        {tab === 'giao-dien' && <AppearanceTab />}
        {tab === 'thong-bao' && <NotificationSettings />}
        <p className="mt-8 text-xs text-ink-muted select-text">{appVersionLabel()}</p>
      </div>
    </section>
  )
}
