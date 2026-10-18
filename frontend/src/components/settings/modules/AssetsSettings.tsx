import { useState } from 'react'
import type { AssetSettings as AssetOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { ASSET_DEFAULTS } from '../../../lib/settings-defaults'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

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
  const { settings, save } = useServerSettings('asset', workspaceId, ASSET_DEFAULTS)
  const [page, setPage] = useState<'tab' | null>(null)
  const done = () => setPage(null)

  if (page === 'tab') return <ChoicePage title="Tab mở đầu" value={settings.default_tab} choices={TABS} onPick={(default_tab) => void save({ default_tab })} onBack={done} />

  return (
    <SettingsPage title="Cài đặt Tài sản" onBack={back}>
      <SettingsGroup title="Hiển thị">
        <SettingsRow label="Tab mở đầu" value={choiceLabel(TABS, settings.default_tab)} onPress={() => setPage('tab')} />
      </SettingsGroup>
    </SettingsPage>
  )
}
