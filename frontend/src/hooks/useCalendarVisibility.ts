import { useCallback, useMemo } from 'react'
import type { CalendarSummary } from '../api/calendar'
import { calendarShown } from '../lib/calendar-format'
import { CALENDAR_DEFAULTS } from '../lib/settings-defaults'
import { useUpdateSubscription } from './useCalendar'
import { useServerSettings } from './useServerSettings'

/**
 * Which calendars Lịch shows, and the one way to change it. A calendar shows when its own checkbox (the
 * subscription's `visible`) is on and the person's saved list of visible calendars (Cài đặt Lịch → Lịch đang hiện)
 * names it; an empty or absent list is every calendar. Showing a calendar the list leaves out also adds it to the
 * list, so a list saved on another device never hides a calendar beyond reach.
 */
export function useCalendarVisibility(workspaceId: string) {
  const { settings, save } = useServerSettings('calendar', workspaceId, CALENDAR_DEFAULTS)
  const update = useUpdateSubscription(workspaceId)
  const listed = useMemo(() => settings.visibleCalendars ?? [], [settings.visibleCalendars])
  const { mutate } = update

  const isShown = useCallback((c: Pick<CalendarSummary, 'id' | 'visible'>) => calendarShown(c, listed), [listed])

  const setShown = useCallback(
    (c: Pick<CalendarSummary, 'id' | 'visible'>, visible: boolean) => {
      if (c.visible !== visible) mutate({ calId: c.id, patch: { visible } })
      if (visible && listed.length > 0 && !listed.includes(c.id)) void save({ visibleCalendars: [...listed, c.id] })
    },
    [listed, mutate, save],
  )

  return { isShown, setShown }
}
