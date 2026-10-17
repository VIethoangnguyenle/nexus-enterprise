import { useCallback } from 'react'
import { keepPreviousData, queryOptions, useMutation, useQuery } from '@tanstack/react-query'
import {
  calendarApi, type CalendarSettings, type EditScope, type EventDetail, type EventPatch,
} from '../api/calendar'
import { toast } from '../components/primitives'
import { notifiesGuests } from '../lib/calendar-editor'
import { toWire } from '../lib/calendar-format'
import { explain, statusOf } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Settings ----------------------------------------------------------------------

export const settingsQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: keys.calendar.settings(wsId),
    queryFn: () => calendarApi.getSettings(wsId),
    enabled: !!wsId,
  })

export function useCalendarSettings(wsId: string) {
  return useQuery(settingsQueryOptions(wsId))
}

/** Saves the zone, working hours and default reminder; new events and Tìm giờ read them. */
export function useUpdateSettings(wsId: string) {
  return useMutation({
    mutationFn: (s: CalendarSettings) => calendarApi.updateSettings(wsId, s),
    onSuccess: (saved) => {
      queryClient.setQueryData(keys.calendar.settings(wsId), saved)
      void queryClient.invalidateQueries({ queryKey: keys.calendar.freeBusyAll(wsId) })
      toast('Đã lưu cài đặt lịch')
    },
    onError: (err) => toast.error(explain(err, 'lưu cài đặt lịch')),
  })
}

// --- Search -----------------------------------------------------------------------

/** Events matching the words, any date; nothing is asked for an empty box. */
export function useSearchEvents(wsId: string, q: string) {
  const term = q.trim()
  return useQuery({
    queryKey: keys.calendar.search(wsId, term),
    queryFn: () => calendarApi.searchEvents(wsId, term),
    enabled: !!wsId && term.length > 0,
    placeholderData: keepPreviousData,
    select: (d) => d.occurrences ?? [],
  })
}

// --- Free/busy ---------------------------------------------------------------------

/** When the named people are busy between `from` and `to`; their nodes only, never shown. */
export function useFreeBusy(wsId: string, users: readonly string[], from: Date | null, to: Date | null) {
  const fromWire = from ? toWire(from) : ''
  const toWireStr = to ? toWire(to) : ''
  return useQuery({
    queryKey: keys.calendar.freeBusy(wsId, users, fromWire, toWireStr),
    queryFn: () => calendarApi.freeBusy(wsId, [...users], fromWire, toWireStr),
    enabled: !!wsId && users.length > 0 && !!from && !!to,
    placeholderData: keepPreviousData,
    select: (d) => d.people ?? [],
  })
}

/** The earliest times in working hours when all the people are free for the length. */
export function useSuggestSlots(wsId: string, users: readonly string[], durationMin: number, day: string) {
  return useQuery({
    queryKey: keys.calendar.suggest(wsId, users, durationMin, day),
    queryFn: () => calendarApi.suggestSlots(wsId, [...users], durationMin, day),
    enabled: !!wsId && users.length > 0 && durationMin > 0 && !!day,
    placeholderData: keepPreviousData,
    select: (d) => d.slots ?? [],
  })
}

// --- Saving ------------------------------------------------------------------------

export type SaveResult = { ok: true; event: EventDetail } | { ok: false; reason: 'conflict' | 'failed' }

const refreshAll = (wsId: string) => {
  void queryClient.invalidateQueries({ queryKey: keys.calendar.eventsAll(wsId) })
  void queryClient.invalidateQueries({ queryKey: keys.calendar.eventDetailsAll(wsId) })
}

/**
 * Saves an edit of an existing event. "Đã lưu. Đã báo 4 khách." only when the change is one guests are told
 * about; a conflict (someone saved first) is returned for the editor to offer the newer version.
 */
export function useSaveEvent(wsId: string) {
  return useCallback(
    async (eventId: string, patch: EventPatch, scope: { scope?: EditScope; occurrence?: string }): Promise<SaveResult> => {
      try {
        const event = await calendarApi.updateEvent(wsId, eventId, patch, scope)
        refreshAll(wsId)
        const told = notifiesGuests(patch) ? event.guestCount : 0
        toast(told > 0 ? `Đã lưu. Đã báo ${told} khách.` : 'Đã lưu')
        return { ok: true, event }
      } catch (err) {
        if (statusOf(err) === 409) return { ok: false, reason: 'conflict' }
        toast.error(explain(err, 'lưu sự kiện'))
        return { ok: false, reason: 'failed' }
      }
    },
    [wsId],
  )
}
