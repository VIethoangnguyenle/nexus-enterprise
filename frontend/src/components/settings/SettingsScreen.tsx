import { useNavigate, useSearch } from '@tanstack/react-router'
import { SETTINGS_TABS, tabOf, tabSearch, type SettingsSearch, type SettingsTab } from '../../lib/settings-search'
import { Heading, TabBar, type TabItem } from '../primitives'
import { AppearanceTab } from './AppearanceTab'
import { ProfileTab } from './ProfileTab'
import { WorkspaceTab } from './WorkspaceTab'

const TABS: TabItem[] = [
  { id: 'ho-so', label: 'Hồ sơ' },
  { id: 'workspace', label: 'Workspace' },
  { id: 'giao-dien', label: 'Giao diện' },
]

/**
 * Cài đặt (design/mockups/contacts-documents-settings.html §4–6): three tabs,
 * the open one kept in the URL (`?tab=giao-dien`) so reload, Back and a link
 * land on it. There is no list panel here.
 */
export function SettingsScreen() {
  const search = useSearch({ strict: false }) as SettingsSearch
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
      <div className="px-5 overflow-x-auto">
        <TabBar className="w-max" label="Cài đặt" idPrefix="settings" value={tab} tabs={TABS} onChange={go} />
      </div>
      <div
        role="tabpanel"
        id={`settings-panel-${tab}`}
        aria-labelledby={`settings-tab-${tab}`}
        className="flex-1 min-h-0 overflow-y-auto px-5 py-5"
      >
        {tab === 'ho-so' && <ProfileTab />}
        {tab === 'workspace' && <WorkspaceTab />}
        {tab === 'giao-dien' && <AppearanceTab />}
      </div>
    </section>
  )
}
