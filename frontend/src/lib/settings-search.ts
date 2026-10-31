import { validateWorkspaceSearch } from './workspace'

/**
 * The tabs Cài đặt once had, as an address from those days names them (`?tab=giao-dien`). Desktop Cài đặt is now a
 * section list at `/settings/<section>`; these names are read only to send such an address on to its section.
 */
export type SettingsTab = 'ho-so' | 'workspace' | 'giao-dien' | 'thong-bao'

const SETTINGS_TABS: readonly SettingsTab[] = ['ho-so', 'workspace', 'giao-dien', 'thong-bao']

export interface SettingsSearch {
  ws?: string
  /** An old tab address. Absent means Hồ sơ. */
  tab?: Exclude<SettingsTab, 'ho-so'>
}

/** `validateSearch` for /settings: an unknown tab opens Hồ sơ rather than an error page. */
export function validateSettingsSearch(search: Record<string, unknown>): SettingsSearch {
  const out: SettingsSearch = { ...validateWorkspaceSearch(search) }
  if (search.tab === 'workspace' || search.tab === 'giao-dien' || search.tab === 'thong-bao') out.tab = search.tab
  return out
}

/**
 * The old tab an address names. The value is checked again here: a search read without a route (`strict: false`)
 * can still carry what a parent route never validated.
 */
export const tabOf = (s: SettingsSearch): SettingsTab =>
  (SETTINGS_TABS as readonly unknown[]).includes(s.tab) ? (s.tab as SettingsTab) : 'ho-so'

/** The `/settings/<section>` that holds an old tab's options now, so an address from the tab days is not broken. */
export function sectionOfTab(tab: SettingsTab): 'account' | 'workspace' | 'appearance' | 'notifications' {
  switch (tab) {
    case 'workspace': return 'workspace'
    case 'giao-dien': return 'appearance'
    case 'thong-bao': return 'notifications'
    default: return 'account'
  }
}
