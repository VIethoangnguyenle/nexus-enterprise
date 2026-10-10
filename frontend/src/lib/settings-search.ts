import { validateWorkspaceSearch } from './workspace'

/** The three tabs of Cài đặt, as they appear in the address (`?tab=giao-dien`). */
export type SettingsTab = 'ho-so' | 'workspace' | 'giao-dien'

export const SETTINGS_TABS: readonly SettingsTab[] = ['ho-so', 'workspace', 'giao-dien']

export interface SettingsSearch {
  ws?: string
  /** Absent means Hồ sơ. */
  tab?: Exclude<SettingsTab, 'ho-so'>
}

/** `validateSearch` for /settings: an unknown tab opens Hồ sơ rather than an error page. */
export function validateSettingsSearch(search: Record<string, unknown>): SettingsSearch {
  const out: SettingsSearch = { ...validateWorkspaceSearch(search) }
  if (search.tab === 'workspace' || search.tab === 'giao-dien') out.tab = search.tab
  return out
}

/**
 * The tab to show. The value is checked again here: a search read without a
 * route (`strict: false`) can still carry what a parent route never validated.
 */
export const tabOf = (s: SettingsSearch): SettingsTab =>
  (SETTINGS_TABS as readonly unknown[]).includes(s.tab) ? (s.tab as SettingsTab) : 'ho-so'

/** Search that opens a tab, keeping the workspace. Hồ sơ is the default and leaves no `tab` behind. */
export function tabSearch(prev: SettingsSearch, tab: SettingsTab): SettingsSearch {
  const { tab: _t, ...rest } = prev
  return tab === 'ho-so' ? rest : { ...rest, tab }
}
