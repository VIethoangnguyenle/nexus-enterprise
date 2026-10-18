import { useState } from 'react'
import type { DriveSettings as DriveOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { DRIVE_DEFAULTS } from '../../../lib/settings-defaults'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

export { DRIVE_DEFAULTS }

const VIEWS: Choice<NonNullable<DriveOptions['view']>>[] = [
  { value: 'auto', label: 'Tự động', hint: 'Danh sách trên điện thoại, lưới trên máy tính' },
  { value: 'list', label: 'Danh sách' },
  { value: 'grid', label: 'Lưới' },
]
const SORTS: Choice<NonNullable<DriveOptions['sort']>>[] = [
  { value: 'modified', label: 'Sửa gần nhất' },
  { value: 'name', label: 'Tên' },
  { value: 'size', label: 'Dung lượng' },
]

/**
 * Cài đặt Tài liệu (DESIGN.md §6, mockup mobile-shell.html §4): how the list shows and previews on mobile data.
 * "Được chia sẻ tài liệu" is stored by the drive service but no notification is sent for a share yet, so the
 * switch is not offered until one is.
 */
export function DriveSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const { settings, save } = useServerSettings('drive', workspaceId, DRIVE_DEFAULTS)
  const [page, setPage] = useState<'view' | 'sort' | null>(null)
  const done = () => setPage(null)

  if (page === 'view') return <ChoicePage title="Xem dạng" value={settings.view} choices={VIEWS} onPick={(view) => void save({ view })} onBack={done} />
  if (page === 'sort') return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onBack={done} />

  return (
    <SettingsPage title="Cài đặt Tài liệu" onBack={back}>
      <SettingsGroup title="Hiển thị">
        <SettingsRow label="Xem dạng" value={choiceLabel(VIEWS, settings.view)} onPress={() => setPage('view')} />
        <SettingsRow label="Sắp xếp" value={choiceLabel(SORTS, settings.sort)} onPress={() => setPage('sort')} />
        <SettingsSwitch label="Thư mục trước tệp" checked={!!settings.folders_first} onChange={(folders_first) => void save({ folders_first })} />
      </SettingsGroup>
      <SettingsGroup title="Dữ liệu di động">
        <SettingsSwitch
          label="Tải ảnh xem trước"
          hint="Tắt: chỉ tải qua Wi-Fi"
          checked={!!settings.cellular_previews}
          onChange={(cellular_previews) => void save({ cellular_previews })}
        />
      </SettingsGroup>
    </SettingsPage>
  )
}
