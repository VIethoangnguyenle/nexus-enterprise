import { useState } from 'react'
import type { ApprovalSettings as ApprovalOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { APPROVAL_DEFAULTS } from '../../../lib/settings-defaults'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsWide } from '../settings-layout'
import { useSettingsNav } from '../settings-nav'
import { useWorkspaceLead } from './workspace-lead'

export { APPROVAL_DEFAULTS }

const SORTS: Choice<NonNullable<ApprovalOptions['sort']>>[] = [
  { value: 'newest', label: 'Mới nhất trước' },
  { value: 'oldest', label: 'Cũ nhất trước' },
]

/**
 * Cài đặt Phê duyệt (DESIGN.md §6, organization workspaces only, kept per organization): notifications by kind of
 * work and the order of Chờ tôi duyệt. A personal workspace has no Phê duyệt and no address for this page.
 *
 * The order applies below 1024px only (a wide screen keeps the server's order, oldest first), so the desktop shell
 * (`useSettingsWide`) does not offer it. "Tab mở đầu" is not offered: the stored choices (chờ duyệt, đã duyệt, đã từ chối) are not the
 * screen's tabs, so it would change nothing.
 */
export function ApprovalSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const wide = useSettingsWide()
  const lead = useWorkspaceLead()
  const { settings, save } = useServerSettings('approval', workspaceId, APPROVAL_DEFAULTS)
  const [page, setPage] = useState<'sort' | null>(null)
  const done = () => setPage(null)

  if (page === 'sort') return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORTS} onPick={(sort) => void save({ sort })} onBack={done} />

  return (
    <SettingsPage title={wide ? 'Phê duyệt' : 'Cài đặt Phê duyệt'} lead={lead} onBack={back}>
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
      {!wide && (
        <SettingsGroup title="Hiển thị">
          <SettingsRow
            label="Sắp xếp"
            hint="Áp dụng cho Chờ tôi duyệt"
            value={choiceLabel(SORTS, settings.sort)}
            onPress={() => setPage('sort')}
          />
        </SettingsGroup>
      )}
    </SettingsPage>
  )
}
