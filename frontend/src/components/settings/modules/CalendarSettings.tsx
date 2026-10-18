import { useMemo, useState } from 'react'
import type { CalendarSettings as CalendarOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useCalendars, useUpdateSubscription } from '../../../hooks/useCalendar'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { zoneLabel, zoneNames } from '../../calendar/editor-ui'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'

/** What the calendar service answers for someone who never saved (its own defaults). */
export const CALENDAR_DEFAULTS: CalendarOptions = {
  tzid: 'Asia/Ho_Chi_Minh', workStart: '08:00', workEnd: '17:30', defaultReminder: 10,
  defaultView: 'agenda', weekStart: 1, defaultDuration: 30,
  notifyInvites: true, notifyChanges: true, notifyReminders: true,
}

const VIEWS: Choice<NonNullable<CalendarOptions['defaultView']>>[] = [
  { value: 'agenda', label: 'Lịch trình' },
  { value: 'day', label: 'Ngày' },
  { value: 'week', label: 'Tuần' },
  { value: 'month', label: 'Tháng' },
]
const WEEK_STARTS: Choice<0 | 1>[] = [
  { value: 1, label: 'Thứ Hai' },
  { value: 0, label: 'Chủ Nhật' },
]
const DURATIONS: Choice<number>[] = [15, 30, 45, 60, 90, 120].map((m) => ({
  value: m, label: m < 60 ? `${m} phút` : m % 60 === 0 ? `${m / 60} giờ` : `${Math.floor(m / 60)} giờ ${m % 60} phút`,
}))
const REMINDERS: Choice<number>[] = [
  { value: 0, label: 'Không nhắc' },
  { value: 5, label: '5 phút' },
  { value: 10, label: '10 phút' },
  { value: 15, label: '15 phút' },
  { value: 30, label: '30 phút' },
  { value: 60, label: '1 giờ' },
  { value: 1440, label: '1 ngày' },
]

/** A saved value outside the list still reads as itself. */
const withValue = (choices: Choice<number>[], v: number | undefined, label: (n: number) => string) =>
  v == null || choices.some((c) => c.value === v) ? choices : [...choices, { value: v, label: label(v) }]

type Page = 'view' | 'week' | 'zone' | 'calendars' | 'duration' | 'reminder' | null

/**
 * Cài đặt Lịch (DESIGN.md §6, mockup mobile-shell.html §4): view, week start, zone and the calendars shown; what a
 * new event starts with; and the three notification switches. Kept in `calendar_user_settings`; Lịch's own
 * settings dialog reads the same value.
 */
export function CalendarSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const { settings, save } = useServerSettings('calendar', workspaceId, CALENDAR_DEFAULTS)
  const calendars = useCalendars(workspaceId)
  const [page, setPage] = useState<Page>(null)
  const done = () => setPage(null)
  const zones = useMemo(() => zoneNames(settings.tzid ?? '').map((z) => ({ value: z, label: zoneLabel(z) })), [settings.tzid])
  const durations = withValue(DURATIONS, settings.defaultDuration, (n) => `${n} phút`)
  const reminders = withValue(REMINDERS, settings.defaultReminder, (n) => `${n} phút`)
  const shown = calendars.data?.filter((c) => c.visible).length

  switch (page) {
    case 'view':
      return <ChoicePage title="Chế độ xem mặc định" value={settings.defaultView} choices={VIEWS} onPick={(defaultView) => void save({ defaultView })} onBack={done} />
    case 'week':
      return <ChoicePage title="Tuần bắt đầu vào" value={settings.weekStart} choices={WEEK_STARTS} onPick={(weekStart) => void save({ weekStart })} onBack={done} />
    case 'zone':
      return <ChoicePage title="Múi giờ" value={settings.tzid} choices={zones} onPick={(tzid) => void save({ tzid })} onBack={done} />
    case 'duration':
      return <ChoicePage title="Thời lượng mặc định" value={settings.defaultDuration} choices={durations} onPick={(defaultDuration) => void save({ defaultDuration })} onBack={done} />
    case 'reminder':
      return (
        <ChoicePage
          title="Nhắc trước"
          value={settings.defaultReminder}
          choices={reminders}
          note="Sự kiện mới bắt đầu với lời nhắc này; đổi riêng từng sự kiện khi tạo."
          onPick={(defaultReminder) => void save({ defaultReminder })}
          onBack={done}
        />
      )
    case 'calendars':
      return <ShownCalendarsPage onBack={done} />
  }

  return (
    <SettingsPage title="Cài đặt Lịch" onBack={back}>
      <SettingsGroup title="Hiển thị">
        <SettingsRow label="Chế độ xem mặc định" value={choiceLabel(VIEWS, settings.defaultView)} onPress={() => setPage('view')} />
        <SettingsRow label="Tuần bắt đầu vào" value={choiceLabel(WEEK_STARTS, settings.weekStart)} onPress={() => setPage('week')} />
        <SettingsRow label="Múi giờ" value={settings.tzid ? zoneLabel(settings.tzid) : undefined} onPress={() => setPage('zone')} />
        <SettingsRow
          label="Lịch đang hiện"
          value={shown == null ? undefined : `${shown} lịch`}
          onPress={calendars.data ? () => setPage('calendars') : undefined}
        />
      </SettingsGroup>
      <SettingsGroup title="Sự kiện mới">
        <SettingsRow label="Thời lượng mặc định" value={choiceLabel(durations, settings.defaultDuration)} onPress={() => setPage('duration')} />
        <SettingsRow label="Nhắc trước" value={choiceLabel(reminders, settings.defaultReminder)} onPress={() => setPage('reminder')} />
      </SettingsGroup>
      <SettingsGroup title="Thông báo">
        <SettingsSwitch label="Lời mời mới" checked={settings.notifyInvites !== false} onChange={(notifyInvites) => void save({ notifyInvites })} />
        <SettingsSwitch label="Thay đổi sự kiện" checked={settings.notifyChanges !== false} onChange={(notifyChanges) => void save({ notifyChanges })} />
        <SettingsSwitch label="Nhắc sự kiện" checked={settings.notifyReminders !== false} onChange={(notifyReminders) => void save({ notifyReminders })} />
      </SettingsGroup>
    </SettingsPage>
  )
}

/** Lịch đang hiện: a switch per calendar, the same one as the calendar's checkbox in Lịch. */
function ShownCalendarsPage({ onBack }: { onBack: () => void }) {
  const { workspaceId } = useActiveWorkspace()
  const calendars = useCalendars(workspaceId)
  const update = useUpdateSubscription(workspaceId)
  return (
    <SettingsPage title="Lịch đang hiện" onBack={onBack}>
      <SettingsGroup note="Lịch tắt ở đây không hiện sự kiện trên lưới Lịch.">
        {(calendars.data ?? []).map((c) => (
          <SettingsSwitch
            key={c.id}
            label={c.name}
            checked={c.visible}
            onChange={(visible) => update.mutate({ calId: c.id, patch: { visible } })}
          />
        ))}
      </SettingsGroup>
    </SettingsPage>
  )
}
