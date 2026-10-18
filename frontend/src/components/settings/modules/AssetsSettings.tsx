import { useState } from 'react'
import type { AssetSettings as AssetOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

/** The asset service's defaults for someone who never saved. */
export const ASSET_DEFAULTS: AssetOptions = { default_tab: 'overview', sort: 'name', notify_requests: true, notify_maintenance: true }

const TABS: Choice<NonNullable<AssetOptions['default_tab']>>[] = [
  { value: 'overview', label: 'Tổng quan' },
  { value: 'inventory', label: 'Danh sách tài sản' },
  { value: 'requests', label: 'Yêu cầu' },
]
const SORTS: Choice<NonNullable<AssetOptions['sort']>>[] = [
  { value: 'name', label: 'Tên' },
  { value: 'updated', label: 'Cập nhật gần nhất' },
  { value: 'created', label: 'Mới thêm' },
  { value: 'status', label: 'Trạng thái' },
]

/** Cài đặt Tài sản (DESIGN.md §6, organization workspaces only): notifications by kind of work, and how it opens. */
export function AssetsSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const { settings, save } = useServerSettings('asset', workspaceId, ASSET_DEFAULTS)
  const [page, setPage] = useState<'tab' | 'sort' | null>(null)
  const done = () => setPage(null)

  if (page === 'tab') return <ChoicePage title="Tab mở đầu" value={settings.default_tab} choices={TABS} onPick={(default_tab) => void save({ default_tab })} onBack={done} />
  if (page === 'sort') return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onBack={done} />

  return (
    <SettingsPage title="Cài đặt Tài sản" onBack={back}>
      <SettingsGroup title="Thông báo">
        <SettingsSwitch
          label="Yêu cầu tài sản"
          hint="Yêu cầu mới cần bạn xử lý, kết quả yêu cầu của bạn"
          checked={!!settings.notify_requests}
          onChange={(notify_requests) => void save({ notify_requests })}
        />
        <SettingsSwitch
          label="Bảo trì"
          hint="Tài sản bạn giữ đến hạn bảo trì"
          checked={!!settings.notify_maintenance}
          onChange={(notify_maintenance) => void save({ notify_maintenance })}
        />
      </SettingsGroup>
      <SettingsGroup title="Hiển thị">
        <SettingsRow label="Tab mở đầu" value={choiceLabel(TABS, settings.default_tab)} onPress={() => setPage('tab')} />
        <SettingsRow label="Sắp xếp" value={choiceLabel(SORTS, settings.sort)} onPress={() => setPage('sort')} />
      </SettingsGroup>
    </SettingsPage>
  )
}
