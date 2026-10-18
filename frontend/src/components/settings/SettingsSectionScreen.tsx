import { Navigate, useParams } from '@tanstack/react-router'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { AccountScreen, OrgProfileScreen } from './AccountScreen'
import { AppearanceScreen } from './AppearanceScreen'
import { NotificationsScreen } from './NotificationsScreen'
import { SessionsScreen } from './SessionsScreen'
import { WorkspaceSettingsScreen } from './WorkspaceSettingsScreen'
import { ApprovalSettings, AssetsSettings, CalendarSettings, ChatSettings, DriveSettings } from './modules'
import { isSettingsSection, type SettingsSection } from './settings-nav'

/** Pages an organization has and a personal workspace does not. */
const ORGANIZATION_ONLY: readonly SettingsSection[] = ['org-profile', 'approval', 'assets']

const PAGES: Record<SettingsSection, () => React.JSX.Element> = {
  account: AccountScreen,
  'org-profile': OrgProfileScreen,
  notifications: NotificationsScreen,
  appearance: AppearanceScreen,
  sessions: SessionsScreen,
  workspace: WorkspaceSettingsScreen,
  chat: ChatSettings,
  calendar: CalendarSettings,
  drive: DriveSettings,
  approval: ApprovalSettings,
  assets: AssetsSettings,
}

/**
 * `/settings/<section>`. An address that names no page, or a page the workspace does not have (Phê duyệt in a
 * personal workspace), lands on Cài đặt rather than on an error.
 */
export function SettingsSectionScreen() {
  const { section } = useParams({ strict: false }) as { section?: string }
  const { isPersonal, isLoading } = useWorkspaceType()
  const keep = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})
  if (!isSettingsSection(section)) return <Navigate to="/settings" search={keep} replace />
  if (ORGANIZATION_ONLY.includes(section)) {
    if (isLoading) return null
    if (isPersonal) return <Navigate to="/settings" search={keep} replace />
  }
  const Page = PAGES[section]
  return <Page />
}
