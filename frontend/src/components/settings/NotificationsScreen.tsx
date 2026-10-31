import { useState } from 'react'
import { AlertTriangle, Bell, BellOff, Calendar, ClipboardCheck, FileText, MessageSquare, Moon, Package } from 'lucide-react'
import type { PushPreferences } from '../../lib/push'
import { usePushDevice } from '../../hooks/usePushDevice'
import { browserZone, isPaused, usePushPreferences } from '../../hooks/usePushPreferences'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { Button, Pressable, Select } from '../primitives'
import { PushBanners } from './PushBanners'
import { SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch } from './SettingsGroup'
import { PAUSE_CHOICES, pauseLabel, pauseUntil } from './pause'
import { ACCOUNT_LEAD } from './settings-model'
import { useSettingsWide } from './settings-layout'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const
const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)
const QUIET_HINT = 'Không báo, trừ khi bạn được nhắc tên (kể cả @Tất cả)'

/**
 * Thông báo (DESIGN.md §6 "Cài đặt", mockup mobile-shell.html §3): push on this device, quiet hours, and one row per
 * module that opens the Thông báo part of that module's page. A group's own level is set in that group. Inside the
 * desktop shell (mockup settings-desktop.html §2) Tạm dừng thông báo comes first, the pause and the quiet hours are
 * chosen in their rows, and a browser that blocks push gets the steps to allow it. Which kinds of event push live in
 * each module's page, not here.
 */
export function NotificationsScreen() {
  const { back, open } = useSettingsNav()
  const wide = useSettingsWide()
  const { isPersonal } = useWorkspaceType()
  const device = usePushDevice()
  const push = usePushPreferences()
  const [quietOpen, setQuietOpen] = useState(false)
  const quiet = push.prefs?.quiet_hours

  if (!wide && quietOpen && push.prefs) return <QuietHoursPage onBack={() => setQuietOpen(false)} />

  return (
    <SettingsPage title="Thông báo" lead={ACCOUNT_LEAD} onBack={back}>
      {wide ? (
        <PushBanners blocked={device.available && device.blocked} ios={device.available && device.ios} />
      ) : (
        <>
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
        </>
      )}

      <SettingsGroup>
        {wide && <PauseRow />}
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
          <div className={wide ? 'py-2' : 'px-4 py-2'}>
            <Button variant="ghost" size="sm" onClick={device.retry}>Thử kết nối lại</Button>
          </div>
        )}
        {wide ? (
          <QuietHoursRow />
        ) : (
          <SettingsRow
            icon={<Moon {...ICON} />}
            label="Giờ yên lặng"
            hint={QUIET_HINT}
            value={!push.prefs ? undefined : quiet?.enabled ? `${quiet.start}–${quiet.end}` : 'Tắt'}
            onPress={push.prefs ? () => setQuietOpen(true) : undefined}
          />
        )}
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

/**
 * Wide: Tạm dừng thông báo as a drop-down of the phone's choices. While paused it shows until when; Không tạm dừng
 * resumes. Only the pause changes: the kinds and the quiet hours are sent as they are.
 */
function PauseRow() {
  const push = usePushPreferences()
  const paused = isPaused(push.prefs)
  const pick = (value: string) => {
    const choice = PAUSE_CHOICES.find((c) => c.value === value)
    if (!choice || (choice.value === 'off' && !paused)) return
    void push.save((p) => ({ ...p, pause_until: pauseUntil(choice.value) }))
  }
  return (
    <SettingsRow
      icon={<BellOff {...ICON} />}
      label="Tạm dừng thông báo"
      hint="Không đẩy thông báo nào tới bạn trong khoảng này."
      trailing={
        <div className="w-60 shrink-0">
          <Select aria-label="Tạm dừng thông báo" value={paused ? 'paused' : 'off'} disabled={!push.prefs} onChange={(e) => pick(e.target.value)}>
            {paused && <option value="paused" disabled>{pauseLabel(push.prefs?.pause_until)}</option>}
            {PAUSE_CHOICES.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
          </Select>
        </div>
      }
    />
  )
}

/**
 * Giờ yên lặng being changed: on or off and the window, always in the browser's own zone so the server applies it
 * where the person is. A start equal to the end is refused here, in words (the server refuses it too).
 */
function useQuietHours() {
  const push = usePushPreferences()
  const [error, setError] = useState<string | null>(null)
  const quiet = push.prefs?.quiet_hours

  const change = (patch: Partial<PushPreferences['quiet_hours']>) => {
    if (!quiet) return
    const next = { ...quiet, ...patch }
    if (next.enabled && next.start === next.end) {
      setError('Giờ bắt đầu và kết thúc không được trùng.')
      return
    }
    setError(null)
    void push.save((p) => ({ ...p, quiet_hours: { ...p.quiet_hours, ...patch, time_zone: browserZone() } }))
  }

  return { quiet, error, change, saving: push.isSaving }
}

/** Wide: the start and the end, then the switch, on one row (mockup settings-desktop.html §2); off dims the times. */
function QuietHoursRow() {
  const { quiet, error, change, saving } = useQuietHours()
  const on = !!quiet?.enabled
  const time = (key: 'start' | 'end', label: string) => (
    <div className="w-24">
      <Select aria-label={label} className="tnum" value={quiet?.[key] ?? ''} disabled={!quiet} onChange={(e) => change({ [key]: e.target.value })}>
        {!quiet && <option value="" disabled />}
        {HOURS.map((h) => <option key={h} value={h}>{h}</option>)}
      </Select>
    </div>
  )
  return (
    <SettingsRow
      icon={<Moon {...ICON} />}
      label="Giờ yên lặng"
      hint={
        <>
          {QUIET_HINT}
          {error && <span role="alert" className="block font-medium text-danger">{error}</span>}
        </>
      }
      trailing={
        <span className="flex items-center gap-4 shrink-0">
          <span className={`flex items-center gap-2 transition-opacity duration-quick ${on ? '' : 'opacity-50'}`}>
            {time('start', 'Bắt đầu')}
            <span aria-hidden="true" className="text-ink-muted">–</span>
            {time('end', 'Kết thúc')}
          </span>
          <Pressable
            role="switch"
            aria-checked={on}
            aria-label="Giờ yên lặng"
            disabled={!quiet || saving}
            onClick={() => change({ enabled: !on })}
            className={`relative shrink-0 w-11 h-6.5 rounded-full before:absolute before:-inset-2.5 before:content-['']
              ${!quiet || saving ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'}`}
          >
            <span aria-hidden="true" className={`absolute inset-0 rounded-full transition-colors duration-press ${on ? 'bg-accent' : 'bg-sunk field-line'}`} />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5.5 h-5.5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press motion-reduce:transition-none ${on ? 'translate-x-4.5' : ''}`}
            />
          </Pressable>
        </span>
      }
    />
  )
}

/** Phone: the page under the Giờ yên lặng row, the switch and, while on, the start and the end. */
function QuietHoursPage({ onBack }: { onBack: () => void }) {
  const { quiet, error, change, saving } = useQuietHours()
  if (!quiet) return null

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
        <SettingsSwitch label="Giờ yên lặng" checked={quiet.enabled} disabled={saving} onChange={(enabled) => change({ enabled })} />
        {quiet.enabled && hourSelect('Bắt đầu', quiet.start, 'start')}
        {quiet.enabled && hourSelect('Kết thúc', quiet.end, 'end')}
      </SettingsGroup>
      {error && <p role="alert" className="m-0 px-1 text-sm text-danger">{error}</p>}
    </SettingsPage>
  )
}
