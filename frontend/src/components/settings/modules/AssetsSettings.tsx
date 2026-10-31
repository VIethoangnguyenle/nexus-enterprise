import { useState } from 'react'
import type { AssetSettings as AssetOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { ASSET_DEFAULTS } from '../../../lib/settings-defaults'
import { ChoicePage, SettingsGroup, SettingsPage, type Choice } from '../SettingsGroup'
import { SettingsChoice } from '../SettingsChoice'
import { useSettingsWide } from '../settings-layout'
import { useSettingsNav } from '../settings-nav'
import { useWorkspaceLead } from './workspace-lead'

export { ASSET_DEFAULTS }

const TABS: Choice<NonNullable<AssetOptions['default_tab']>>[] = [
  { value: 'overview', label: 'Tổng quan' },
  { value: 'inventory', label: 'Danh sách tài sản' },
  { value: 'requests', label: 'Yêu cầu' },
]
/**
 * Cài đặt Tài sản (DESIGN.md §6, organization workspaces only): the tab Tài sản opens on. The service also stores
 * "Yêu cầu tài sản" and "Bảo trì" notification switches and a list order, but it sends no such notification and
 * its list has no order to choose yet, so those rows are not offered until they would change something.
 */
export function AssetsSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const wide = useSettingsWide()
  const lead = useWorkspaceLead()
  const { settings, save } = useServerSettings('asset', workspaceId, ASSET_DEFAULTS)
  const [page, setPage] = useState<'tab' | null>(null)
  const done = () => setPage(null)

  if (page === 'tab') return <ChoicePage title="Tab mở đầu" value={settings.default_tab} choices={TABS} onPick={(default_tab) => void save({ default_tab })} onBack={done} />

  return (
    <SettingsPage title={wide ? 'Tài sản' : 'Cài đặt Tài sản'} lead={lead} onBack={back}>
      <SettingsGroup title="Hiển thị">
        <SettingsChoice
          label="Tab mở đầu"
          value={settings.default_tab}
          choices={TABS}
          onPick={(default_tab) => void save({ default_tab })}
          onOpen={() => setPage('tab')}
        />
      </SettingsGroup>
    </SettingsPage>
  )
}
