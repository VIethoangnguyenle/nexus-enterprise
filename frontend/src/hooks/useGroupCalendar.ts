import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { calendarApi, type CalendarSummary } from '../api/calendar'
import { toast } from '../components/primitives'
import { explain } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { calendarsQueryOptions, useCalendars } from './useCalendar'
import { keys } from './keys'

/** The calendar of a group conversation among the caller's calendars, when it has been made. */
export function useGroupCalendar(wsId: string, channelId: string) {
  const calendars = useCalendars(wsId)
  const calendar: CalendarSummary | undefined = calendars.data?.find((c) => c.kind === 'channel' && c.channel?.id === channelId)
  return { calendar, query: calendars }
}

/**
 * Opens the event editor with the group's calendar chosen. The calendar of a group is made the first time an
 * event is created in it, so this asks the server for it when the caller's list does not hold it yet. A
 * refusal (not in the group any more, a personal workspace) is told and the editor does not open.
 */
export function useNewGroupEvent(wsId: string, channelId: string) {
  const navigate = useNavigate()
  const ensure = useMutation({
    meta: { silentError: true },
    mutationFn: () => calendarApi.ensureChannelCalendar(wsId, channelId),
  })
  const open = useCallback(async () => {
    if (!wsId) return
    try {
      // The list the caller already has says whether the group's calendar exists; only a missing one is asked for.
      const known = (await queryClient.ensureQueryData(calendarsQueryOptions(wsId))).calendars
      let cal = known.find((c) => c.kind === 'channel' && c.channel?.id === channelId)
      if (!cal) {
        cal = await ensure.mutateAsync()
        void queryClient.invalidateQueries({ queryKey: keys.calendar.calendars(wsId) })
      }
      await navigate({ to: '/calendar/new', search: (prev: { ws?: string }) => ({ ws: prev.ws, calendar: cal.id }) })
    } catch (err) {
      toast.error(explain(err, 'tạo sự kiện cho nhóm'))
    }
  }, [channelId, ensure, navigate, wsId])
  return { open, pending: ensure.isPending }
}
