import { useCallback, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { CalendarSettings as CalendarOptions } from '../../../api/settings'
import { keys } from '../../../hooks/keys'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useCalendars } from '../../../hooks/useCalendar'
import { useCalendarVisibility } from '../../../hooks/useCalendarVisibility'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { calendarShown } from '../../../lib/calendar-format'
import { colorStyle } from '../../../lib/calendar-color'
import { CALENDAR_DEFAULTS } from '../../../lib/settings-defaults'
import { zoneLabel, zoneNames } from '../../calendar/editor-ui'
import { Checkbox } from '../../primitives'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { SettingsChoice } from '../SettingsChoice'
import { useSettingsWide } from '../settings-layout'
import { useSettingsNav } from '../settings-nav'
import { WorkHoursPage, WorkHoursRow, workHoursLabel } from './CalendarWorkHours'
import { useWorkspaceLead } from './workspace-lead'

export { CALENDAR_DEFAULTS }

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

type Page = 'view' | 'week' | 'zone' | 'hours' | 'calendars' | 'duration' | 'reminder' | null

/**
 * Cài đặt Lịch (DESIGN.md §6, mockup mobile-shell.html §4; desktop: settings-desktop.html §4): view, week start,
 * zone, working hours and the calendars shown; what a new event starts with; and the three notification switches.
 * Kept in `calendar_user_settings`. This is Lịch's only settings place: the ⚙ in Lịch opens it.
 *
 * The default view and duration apply below 1024px only (a wide screen keeps opening on its week and making
 * one-hour events), so the desktop shell does not offer them.
 */
export function CalendarSettings() {
  const { back } = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const wide = useSettingsWide()
  const lead = useWorkspaceLead()
  const { settings, save } = useServerSettings('calendar', workspaceId, CALENDAR_DEFAULTS)
  const calendars = useCalendars(workspaceId)
  const queryClient = useQueryClient()
  const [page, setPage] = useState<Page>(null)
  const done = () => setPage(null)
  const zones = useMemo(() => zoneNames(settings.tzid ?? '').map((z) => ({ value: z, label: zoneLabel(z) })), [settings.tzid])
  const durations = withValue(DURATIONS, settings.defaultDuration, (n) => `${n} phút`)
  const reminders = withValue(REMINDERS, settings.defaultReminder, (n) => `${n} phút`)
  const shown = calendars.data?.filter((c) => calendarShown(c, settings.visibleCalendars)).length
  // Tìm giờ suggests times inside everyone's working hours, the person's own included: a new pair refreshes it.
  const saveHours = useCallback(async (patch: CalendarOptions) => {
    const saved = await save(patch)
    if (saved) void queryClient.invalidateQueries({ queryKey: keys.calendar.freeBusyAll(workspaceId) })
    return saved
  }, [save, queryClient, workspaceId])

  switch (page) {
    case 'view':
      return <ChoicePage title="Chế độ xem mặc định" value={settings.defaultView} choices={VIEWS} onPick={(defaultView) => void save({ defaultView })} onBack={done} />
    case 'week':
      return <ChoicePage title="Tuần bắt đầu vào" value={settings.weekStart} choices={WEEK_STARTS} onPick={(weekStart) => void save({ weekStart })} onBack={done} />
    case 'zone':
      return <ChoicePage title="Múi giờ" value={settings.tzid} choices={zones} onPick={(tzid) => void save({ tzid })} onBack={done} />
    case 'hours':
      return <WorkHoursPage settings={settings} save={saveHours} onBack={done} />
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
    <SettingsPage title={wide ? 'Lịch' : 'Cài đặt Lịch'} lead={lead} onBack={back}>
      <SettingsGroup title="Hiển thị">
        {!wide && <SettingsRow label="Chế độ xem mặc định" value={choiceLabel(VIEWS, settings.defaultView)} onPress={() => setPage('view')} />}
        <SettingsChoice label="Tuần bắt đầu vào" value={settings.weekStart} choices={WEEK_STARTS} onPick={(weekStart) => void save({ weekStart })} onOpen={() => setPage('week')} />
        <SettingsChoice label="Múi giờ" value={settings.tzid} choices={zones} onPick={(tzid) => void save({ tzid })} onOpen={() => setPage('zone')} />
        {wide
          ? <WorkHoursRow settings={settings} save={saveHours} />
          : <SettingsRow label="Giờ làm việc" value={workHoursLabel(settings)} onPress={() => setPage('hours')} />}
        {wide ? (
          <ShownCalendarsRow />
        ) : (
          <SettingsRow
            label="Lịch đang hiện"
            value={shown == null ? undefined : `${shown} lịch`}
            onPress={calendars.data ? () => setPage('calendars') : undefined}
          />
        )}
      </SettingsGroup>
      <SettingsGroup title="Sự kiện mới">
        {!wide && <SettingsRow label="Thời lượng mặc định" value={choiceLabel(durations, settings.defaultDuration)} onPress={() => setPage('duration')} />}
        <SettingsChoice label="Nhắc trước" value={settings.defaultReminder} choices={reminders} onPick={(defaultReminder) => void save({ defaultReminder })} onOpen={() => setPage('reminder')} />
      </SettingsGroup>
      <SettingsGroup title="Thông báo">
        <SettingsSwitch label="Lời mời mới" checked={settings.notifyInvites !== false} onChange={(notifyInvites) => void save({ notifyInvites })} />
        <SettingsSwitch label="Thay đổi sự kiện" checked={settings.notifyChanges !== false} onChange={(notifyChanges) => void save({ notifyChanges })} />
        <SettingsSwitch label="Nhắc sự kiện" checked={settings.notifyReminders !== false} onChange={(notifyReminders) => void save({ notifyReminders })} />
      </SettingsGroup>
    </SettingsPage>
  )
}

/** Phone: Lịch đang hiện, a switch per calendar, the same one as the calendar's checkbox in Lịch (`useCalendarVisibility`). */
function ShownCalendarsPage({ onBack }: { onBack: () => void }) {
  const { workspaceId } = useActiveWorkspace()
  const calendars = useCalendars(workspaceId)
  const { isShown, setShown } = useCalendarVisibility(workspaceId)
  return (
    <SettingsPage title="Lịch đang hiện" onBack={onBack}>
      <SettingsGroup note="Lịch tắt ở đây không hiện sự kiện trên lưới Lịch.">
        {(calendars.data ?? []).map((c) => (
          <SettingsSwitch
            key={c.id}
            label={c.name}
            checked={isShown(c)}
            onChange={(visible) => setShown(c, visible)}
          />
        ))}
      </SettingsGroup>
    </SettingsPage>
  )
}

/**
 * Wide: Lịch đang hiện as checkboxes in the row, each in its calendar's colour (mockup settings-desktop.html §4,
 * `.cals`); the same visibility as the checkboxes of Lịch's side column.
 */
function ShownCalendarsRow() {
  const { workspaceId } = useActiveWorkspace()
  const calendars = useCalendars(workspaceId)
  const { isShown, setShown } = useCalendarVisibility(workspaceId)
  return (
    <div className="grid gap-2 py-3.5">
      <span className="grid gap-0.5">
        <span className="text-[0.9375rem] font-semibold leading-snug text-ink">Lịch đang hiện</span>
        <span className="text-[0.8125rem] leading-snug text-ink-muted">Bỏ chọn để ẩn sự kiện của lịch đó.</span>
      </span>
      <div role="group" aria-label="Lịch đang hiện" className="grid gap-2 pt-1">
        {(calendars.data ?? []).map((c) => (
          <label key={c.id} style={colorStyle(c.color)} className="flex items-center gap-2.5 min-h-8 text-[0.9375rem] text-ink cursor-pointer">
            <Checkbox label={c.name} checked={isShown(c)} onChange={(e) => setShown(c, e.target.checked)} className="shrink-0" />
            <span aria-hidden="true" className="size-2.5 shrink-0 rounded-sm bg-(--c)" />
            <span className="truncate">{c.name}</span>
          </label>
        ))}
      </div>
    </div>
  )
}
