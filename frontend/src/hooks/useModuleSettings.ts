import type { SettingsByService } from '../api/settings'
import {
  APPROVAL_DEFAULTS, ASSET_DEFAULTS, CALENDAR_DEFAULTS, CHAT_DEFAULTS, DRIVE_DEFAULTS,
} from '../lib/settings-defaults'
import { useBottomBarLayout } from './usePhone'
import { useServerSettings } from './useServerSettings'

type ModuleService = 'chat' | 'drive' | 'asset' | 'calendar' | 'approval'

const DEFAULTS: { [S in ModuleService]: SettingsByService[S] } = {
  chat: CHAT_DEFAULTS,
  drive: DRIVE_DEFAULTS,
  asset: ASSET_DEFAULTS,
  calendar: CALENDAR_DEFAULTS,
  approval: APPROVAL_DEFAULTS,
}

/**
 * The options a person saved for one module in a workspace, as the module's screen obeys them. Every key
 * always has a value: the service's default stands in while the settings load, when the read fails and for a
 * key never set, so a screen never waits on them to draw.
 */
export function useModuleSettings<S extends ModuleService>(service: S, workspaceId: string): Required<SettingsByService[S]> {
  const { settings } = useServerSettings(service, workspaceId, DEFAULTS[service])
  return settings as Required<SettingsByService[S]>
}

/**
 * Whether a plain Enter sends in the composer. Below 1024px it is the person's "Enter để gửi" (off by default:
 * Enter makes a new line, as on a phone keyboard); a wide screen always sends on Enter, whatever the setting, so
 * the desktop keeps the keys it has. The other key (Shift+Enter) does the opposite.
 */
export function useEnterSends(workspaceId: string): boolean {
  const compact = useBottomBarLayout()
  const { enter_to_send } = useModuleSettings('chat', compact ? workspaceId : '')
  return !compact || enter_to_send
}
