import { useState } from 'react'
import { AlertTriangle, Bell, Calendar, ClipboardCheck, FileText, MessageSquare, Moon, Package } from 'lucide-react'
import type { PushPreferences } from '../../lib/push'
import { usePushDevice } from '../../hooks/usePushDevice'
import { browserZone, usePushPreferences } from '../../hooks/usePushPreferences'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { Button, Select } from '../primitives'
import { SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch } from './SettingsGroup'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const
const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)

/**
 * Tôi → Thông báo (DESIGN.md §6 "Cài đặt", mockup mobile-shell.html §3): push on this device, quiet hours, and one
 * row per module that opens the Thông báo part of that module's page. A group's own level is set in that group.
 */
export function NotificationsScreen() {
  const { back, open } = useSettingsNav()
  const { isPersonal } = useWorkspaceType()
  const device = usePushDevice()
  const push = usePushPreferences()
  const [quietOpen, setQuietOpen] = useState(false)
  const quiet = push.prefs?.quiet_hours

  if (quietOpen && push.prefs) return <QuietHoursPage onBack={() => setQuietOpen(false)} />

  return (
    <SettingsPage title="Thông báo" onBack={back}>
      {device.available && device.blocked && (
        <p role="status" className="m-0 p-3.5 rounded-xl bg-danger-wash text-sm text-ink">
          <b className="font-semibold text-danger">Trình duyệt đang chặn thông báo của Nexus.</b> Mở cài đặt của trình duyệt,
          cho phép Thông báo, rồi quay lại đây.
        </p>
      )}
      {device.available && device.ios && (
        <p role="status" className="m-0 p-3.5 rounded-xl bg-info-wash text-sm font-semibold text-ink">
          Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.
        </p>
      )}

      <SettingsGroup>
        {device.available ? (
          <SettingsSwitch
            icon={<Bell {...ICON} />}
            label="Thông báo đẩy trên máy này"
            hint={device.status}
            checked={device.optedIn ?? false}
            disabled={device.disabled}
            onChange={() => void device.toggle()}
          />
        ) : (
          <SettingsRow icon={<Bell {...ICON} />} label="Thông báo đẩy trên máy này" hint="Chưa có trên môi trường này" disabled />
        )}
        {device.failed && !device.blocked && !device.ios && (
          <div className="px-4 py-2">
            <Button variant="ghost" size="sm" onClick={device.retry}>Thử kết nối lại</Button>
          </div>
        )}
        <SettingsRow
          icon={<Moon {...ICON} />}
          label="Giờ yên lặng"
          hint="Không báo, trừ khi bạn được nhắc tên (kể cả @Tất cả)"
          value={!push.prefs ? undefined : quiet?.enabled ? `${quiet.start}–${quiet.end}` : 'Tắt'}
          onPress={push.prefs ? () => setQuietOpen(true) : undefined}
        />
      </SettingsGroup>

      {push.isError && (
        <p role="alert" className="m-0 flex items-center gap-2 px-1 text-sm text-ink-muted">
          <AlertTriangle size={16} strokeWidth={1.75} className="text-danger shrink-0" aria-hidden="true" />
          Không tải được cài đặt thông báo.
          <Button variant="ghost" size="sm" onClick={() => void push.refetch()}>Thử lại</Button>
        </p>
      )}

      <SettingsGroup title="Theo phân hệ" note="Mức thông báo của từng nhóm đặt trong cài đặt nhóm đó.">
        <SettingsRow icon={<MessageSquare {...ICON} />} label="Chat" hint="Tin nhắn trực tiếp, được nhắc tên, thread, cuộc gọi" onPress={() => open('chat')} />
        <SettingsRow icon={<Calendar {...ICON} />} label="Lịch" hint="Lời mời, thay đổi và nhắc sự kiện" onPress={() => open('calendar')} />
        {!isPersonal && (
          <SettingsRow icon={<ClipboardCheck {...ICON} />} label="Phê duyệt" hint="Đề nghị chờ bạn duyệt, kết quả đề nghị của bạn" onPress={() => open('approval')} />
        )}
        <SettingsRow icon={<FileText {...ICON} />} label="Tài liệu" hint="Được chia sẻ tài liệu" onPress={() => open('drive')} />
        {!isPersonal && (
          <SettingsRow icon={<Package {...ICON} />} label="Tài sản" hint="Yêu cầu tài sản, bảo trì" onPress={() => open('assets')} />
        )}
      </SettingsGroup>
    </SettingsPage>
  )
}

/** Giờ yên lặng: on or off, and the window, in the browser's own zone. A start equal to the end is refused here. */
function QuietHoursPage({ onBack }: { onBack: () => void }) {
  const push = usePushPreferences()
  const [error, setError] = useState<string | null>(null)
  const quiet = push.prefs!.quiet_hours

  const change = (patch: Partial<PushPreferences['quiet_hours']>) => {
    const next = { ...quiet, ...patch }
    if (next.enabled && next.start === next.end) {
      setError('Giờ bắt đầu và kết thúc không được trùng.')
      return
    }
    setError(null)
    void push.save((p) => ({ ...p, quiet_hours: { ...p.quiet_hours, ...patch, time_zone: browserZone() } }))
  }

  const hourSelect = (label: string, value: string, key: 'start' | 'end') => (
    <SettingsRow
      label={label}
      trailing={
        <Select
          aria-label={label}
          value={value}
          onChange={(e) => change({ [key]: e.target.value })}
          className="h-10 px-2 rounded-lg bg-sunk text-[0.9375rem] font-medium text-ink focus-ring"
        >
          {HOURS.map((h) => <option key={h} value={h}>{h}</option>)}
        </Select>
      }
    />
  )

  return (
    <SettingsPage title="Giờ yên lặng" onBack={onBack}>
      <SettingsGroup note="Trong giờ yên lặng không có thông báo đẩy, trừ tin nhắn nhắc tên bạn (kể cả @Tất cả). Cuộc gọi và Lịch cũng im.">
        <SettingsSwitch label="Giờ yên lặng" checked={quiet.enabled} disabled={push.isSaving} onChange={(enabled) => change({ enabled })} />
        {quiet.enabled && hourSelect('Bắt đầu', quiet.start, 'start')}
        {quiet.enabled && hourSelect('Kết thúc', quiet.end, 'end')}
      </SettingsGroup>
      {error && <p role="alert" className="m-0 px-1 text-sm text-danger">{error}</p>}
    </SettingsPage>
  )
}
