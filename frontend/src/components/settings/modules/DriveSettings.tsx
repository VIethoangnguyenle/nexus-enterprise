import { useState } from 'react'
import type { DriveSettings as DriveOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { DRIVE_DEFAULTS } from '../../../lib/settings-defaults'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsSwitch, type Choice } from '../SettingsGroup'
import { SettingsChoice } from '../SettingsChoice'
import { useSettingsWide } from '../settings-layout'
import { useSettingsNav } from '../settings-nav'
import { useWorkspaceLead } from './workspace-lead'

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
 * switch is not offered until one is. Mobile data means a phone, so the desktop shell (`useSettingsWide`) does not
 * offer Tải ảnh xem trước (DESIGN.md §6 "Cài đặt trên máy tính").
 */
export function DriveSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const wide = useSettingsWide()
  const lead = useWorkspaceLead()
  const { settings, save } = useServerSettings('drive', workspaceId, DRIVE_DEFAULTS)
  const [page, setPage] = useState<'view' | 'sort' | null>(null)
  const done = () => setPage(null)

  if (page === 'view') return <ChoicePage title="Xem dạng" value={settings.view} choices={VIEWS} onPick={(view) => void save({ view })} onBack={done} />
  if (page === 'sort') return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onBack={done} />

  return (
    <SettingsPage title={wide ? 'Tài liệu' : 'Cài đặt Tài liệu'} lead={lead} onBack={back}>
      <SettingsGroup title="Hiển thị">
        <SettingsChoice label="Xem dạng" value={settings.view} choices={VIEWS} onPick={(view) => void save({ view })} onOpen={() => setPage('view')} />
        <SettingsChoice label="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onOpen={() => setPage('sort')} />
        <SettingsSwitch label="Thư mục trước tệp" checked={!!settings.folders_first} onChange={(folders_first) => void save({ folders_first })} />
      </SettingsGroup>
      {!wide && (
        <SettingsGroup title="Dữ liệu di động">
          <SettingsSwitch
            label="Tải ảnh xem trước"
            hint="Tắt: chỉ tải qua Wi-Fi"
            checked={!!settings.cellular_previews}
            onChange={(cellular_previews) => void save({ cellular_previews })}
          />
        </SettingsGroup>
      )}
    </SettingsPage>
  )
}
