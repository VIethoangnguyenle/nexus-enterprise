import { useState } from 'react'
import type { DriveSettings as DriveOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

/** The drive service's defaults for someone who never saved. */
export const DRIVE_DEFAULTS: DriveOptions = {
  view: 'auto', sort: 'modified', folders_first: true, cellular_previews: true, notify_shared: true,
}

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
 * Cài đặt Tài liệu (DESIGN.md §6, mockup mobile-shell.html §4): how the list shows, previews on mobile data, and the
 * one notification documents have (shared with you; there are no document comments).
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
      <SettingsGroup title="Thông báo">
        <SettingsSwitch label="Được chia sẻ tài liệu" checked={!!settings.notify_shared} onChange={(notify_shared) => void save({ notify_shared })} />
      </SettingsGroup>
    </SettingsPage>
  )
}
