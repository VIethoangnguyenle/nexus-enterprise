import { useState } from 'react'
import type { ApprovalSettings as ApprovalOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

/** The approval service's defaults for someone who never saved. */
export const APPROVAL_DEFAULTS: ApprovalOptions = { default_tab: 'pending', sort: 'newest', notify_pending: true, notify_result: true }

const TABS: Choice<NonNullable<ApprovalOptions['default_tab']>>[] = [
  { value: 'pending', label: 'Chờ tôi duyệt' },
  { value: 'approved', label: 'Đã duyệt' },
  { value: 'rejected', label: 'Đã từ chối' },
]
const SORTS: Choice<NonNullable<ApprovalOptions['sort']>>[] = [
  { value: 'newest', label: 'Mới nhất trước' },
  { value: 'oldest', label: 'Cũ nhất trước' },
]

/**
 * Cài đặt Phê duyệt (DESIGN.md §6, organization workspaces only, kept per organization): notifications by kind of
 * work and how the list opens. A personal workspace has no Phê duyệt and no address for this page.
 */
export function ApprovalSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const { settings, save } = useServerSettings('approval', workspaceId, APPROVAL_DEFAULTS)
  const [page, setPage] = useState<'tab' | 'sort' | null>(null)
  const done = () => setPage(null)

  if (page === 'tab') return <ChoicePage title="Tab mở đầu" value={settings.default_tab} choices={TABS} onPick={(default_tab) => void save({ default_tab })} onBack={done} />
  if (page === 'sort') return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onBack={done} />

  return (
    <SettingsPage title="Cài đặt Phê duyệt" onBack={back}>
      <SettingsGroup title="Thông báo">
        <SettingsSwitch
          label="Đề nghị chờ bạn duyệt"
          checked={!!settings.notify_pending}
          onChange={(notify_pending) => void save({ notify_pending })}
        />
        <SettingsSwitch
          label="Kết quả đề nghị của bạn"
          hint="Khi đề nghị bạn gửi được duyệt hoặc bị từ chối"
          checked={!!settings.notify_result}
          onChange={(notify_result) => void save({ notify_result })}
        />
      </SettingsGroup>
      <SettingsGroup title="Hiển thị">
        <SettingsRow label="Tab mở đầu" value={choiceLabel(TABS, settings.default_tab)} onPress={() => setPage('tab')} />
        <SettingsRow label="Sắp xếp" value={choiceLabel(SORTS, settings.sort)} onPress={() => setPage('sort')} />
      </SettingsGroup>
    </SettingsPage>
  )
}
