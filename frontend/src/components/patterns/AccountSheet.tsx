import { Bell, LogOut, Settings, User } from 'lucide-react'
import { logoutSession } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { Avatar } from '../primitives'
import { NavSheet, SheetSeparator } from '../spaces/NavSheet'
import { PhoneSheetRow } from './PhoneSheetRow'

/**
 * The sheet under the avatar of the phone search bar (DESIGN.md §5, Điều hướng
 * di động): who the person is here, the way to the notification settings, to
 * Cài đặt, and Đăng xuất, which goes through the one logout path. Render it
 * only while open.
 */
export function AccountSheet({ onClose }: { onClose: () => void }) {
  const user = useAuthStore((s) => s.user)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const me = user?.id ? people.byUserId.get(user.id) : undefined
  const name = me?.name || UNKNOWN_PERSON
  const where = workspaceDisplayName(workspaceName)

  const toSettings = () => { setActiveModule('settings'); onClose() }

  return (
    <NavSheet label="Tài khoản" onClose={onClose}>
      <div className="grid grid-cols-[3.25rem_minmax(0,1fr)] items-center gap-3.5 px-3.5 pt-1.5 pb-3">
        <Avatar name={name} hueKey={user?.id} src={me?.avatarUrl} size={52} />
        <span className="grid min-w-0 justify-items-start gap-0.5">
          <b className="text-[1.0625rem] font-semibold text-ink truncate max-w-full">{name}</b>
          <span className="text-sm text-ink-muted truncate max-w-full">{me?.role ? `${me.role} · ${where}` : where}</span>
          <span className="inline-flex items-center gap-1.5 h-7 px-2.5 mt-1 rounded-full bg-success-wash text-[0.8125rem] font-semibold text-ink">
            <span aria-hidden="true" className="w-2 h-2 rounded-full bg-success" />
            Đang hoạt động
          </span>
        </span>
      </div>
      <PhoneSheetRow
        icon={<Bell size={20} strokeWidth={1.75} />}
        hint="Giờ yên lặng và các loại thông báo"
        to="/settings"
        search={{ tab: 'thong-bao' }}
        onClick={toSettings}
      >
        Cài đặt thông báo
      </PhoneSheetRow>
      <PhoneSheetRow icon={<User size={20} strokeWidth={1.75} />} to="/settings" onClick={toSettings}>
        Cài đặt tài khoản
      </PhoneSheetRow>
      <PhoneSheetRow icon={<Settings size={20} strokeWidth={1.75} />} to="/settings" onClick={toSettings}>
        Cài đặt
      </PhoneSheetRow>
      <SheetSeparator />
      <PhoneSheetRow
        icon={<LogOut size={20} strokeWidth={1.75} />}
        tone="danger"
        onClick={() => { onClose(); void logoutSession() }}
      >
        Đăng xuất
      </PhoneSheetRow>
    </NavSheet>
  )
}
