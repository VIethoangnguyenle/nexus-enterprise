import { useCallback, useMemo } from 'react'
import { keepPreviousData, queryOptions, useMutation, useQuery } from '@tanstack/react-query'
import {
  calendarApi,
  type CalendarSummary,
  type EditScope,
  type EventDetail,
  type EventInput,
  type Occurrence,
  type RsvpInput,
  type SubscriptionPatch,
} from '../api/calendar'
import { toast } from '../components/primitives'
import { answerInstances, instanceKey, moveInstance, removeInstances } from '../lib/calendar-cache'
import { movedToast, spanOf, toWire, wireOf } from '../lib/calendar-format'
import { explain, statusOf } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Queries -----------------------------------------------------------------

export const calendarsQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: keys.calendar.calendars(wsId),
    queryFn: () => calendarApi.listCalendars(wsId),
    enabled: !!wsId,
    select: (d) => d.calendars ?? [],
  })

/** The calendars of the side list, in the order the server gives them. */
export function useCalendars(wsId: string) {
  return useQuery(calendarsQueryOptions(wsId))
}

/** How long Hoàn tác is offered after a delete (DESIGN.md §6, Lặp lại). */
export const DELETE_UNDO_MS = 5000

/** Instances whose delete is waiting out its Hoàn tác window; a refetch in that time must not bring them back. */
const pendingRemovals = new Set<string>()

/**
 * Occurrences of `[from, to)` on the named calendars. The previous range stays on screen while the next one
 * loads, so moving from week to week does not flash a skeleton.
 */
export function useOccurrences(wsId: string, from: Date, to: Date, calendarIds: readonly string[]) {
  const fromWire = toWire(from)
  const toWireStr = toWire(to)
  return useQuery({
    queryKey: keys.calendar.events(wsId, fromWire, toWireStr, calendarIds),
    queryFn: () => calendarApi.listEvents(wsId, fromWire, toWireStr, [...calendarIds]),
    enabled: !!wsId && calendarIds.length > 0,
    placeholderData: keepPreviousData,
    select: (d) => (d.occurrences ?? []).filter((o) => !pendingRemovals.has(instanceKey(o))),
  })
}

export const eventQueryOptions = (wsId: string, id: string, occurrence?: string) =>
  queryOptions({
    queryKey: keys.calendar.event(wsId, id, occurrence),
    queryFn: () => calendarApi.getEvent(wsId, id, occurrence),
    enabled: !!wsId && !!id,
    // 403 and 404 are answers ("not yours any more", "gone"), not hiccups.
    retry: (count, err) => ![403, 404].includes(statusOf(err) ?? 0) && count < 1,
  })

/** The whole event behind an occurrence, for the popover; only asked for once it is open. */
export function useEventDetail(wsId: string, id: string | undefined) {
  return useQuery({ ...eventQueryOptions(wsId, id ?? ''), enabled: !!wsId && !!id })
}

// --- Cache plumbing -----------------------------------------------------------

type EventList = { occurrences: Occurrence[] }
type Snapshot = [readonly unknown[], EventList | undefined][]

/** Applies `fn` to every cached event list of the workspace and returns what was there, to put back. */
function patchLists(wsId: string, fn: (list: EventList | undefined) => EventList | undefined): Snapshot {
  const prefix = keys.calendar.eventsAll(wsId)
  const before = queryClient.getQueriesData<EventList>({ queryKey: prefix }) as Snapshot
  for (const [key, list] of before) queryClient.setQueryData<EventList>(key, fn(list))
  return before
}

function restore(snapshot: Snapshot) {
  for (const [key, list] of snapshot) queryClient.setQueryData(key, list)
}

const refresh = (wsId: string) => {
  void queryClient.invalidateQueries({ queryKey: keys.calendar.eventsAll(wsId) })
  void queryClient.invalidateQueries({ queryKey: keys.calendar.eventDetailsAll(wsId) })
}

// --- Writes -------------------------------------------------------------------

/** The scope a change to this occurrence needs: none for a single event. */
export const scopeFor = (o: Pick<Occurrence, 'isRecurring'>, chosen: EditScope): EditScope | undefined =>
  o.isRecurring ? chosen : undefined

export interface MoveInput {
  occurrence: Occurrence
  start: Date
  end: Date
  /** Required for a recurring event: the person's answer in the scope dialog. */
  scope?: EditScope
}

/**
 * Everything the grid, popover and shortcuts change. A move or resize shows at once and is put back with an
 * error toast if the server refuses; the toast offers Hoàn tác, which sends the old times back. A delete
 * disappears at once and is only sent when its Hoàn tác window has passed.
 */
export function useCalendarActions(wsId: string) {
  const move = useCallback(
    async ({ occurrence: o, start, end, scope }: MoveInput) => {
      const oldStart = spanOf(o).start
      const newStart = wireOf(start, o.allDay)
      const newEnd = wireOf(end, o.allDay)
      await queryClient.cancelQueries({ queryKey: keys.calendar.eventsAll(wsId) })
      const snapshot = patchLists(wsId, (l) => moveInstance(l, o, newStart, newEnd))
      try {
        const saved = await calendarApi.updateEvent(
          wsId, o.eventId, { start: newStart, end: newEnd, allDay: o.allDay, version: o.version },
          scope ? { scope, occurrence: o.occurrenceStart } : {},
        )
        toast(movedToast(oldStart, start, o.allDay), {
          action: { label: 'Hoàn tác', onClick: () => void undoMove(wsId, o, saved, scope, newStart) },
        })
      } catch (err) {
        restore(snapshot)
        toast.error(
          statusOf(err) === 409
            ? 'Sự kiện vừa được sửa ở nơi khác. Đã tải lại bản mới.'
            : explain(err, 'dời sự kiện'),
        )
      } finally {
        refresh(wsId)
      }
    },
    [wsId],
  )

  const create = useCallback(
    async (input: EventInput): Promise<EventDetail | null> => {
      try {
        const saved = await calendarApi.createEvent(wsId, input)
        refresh(wsId)
        const invited = saved.guestCount
        toast(invited > 0 ? `Đã tạo sự kiện. Đã gửi lời mời cho ${invited} người` : 'Đã tạo sự kiện', {
          action: {
            label: 'Hoàn tác',
            onClick: () => {
              void calendarApi.deleteEvent(wsId, saved.eventId).catch((err) => toast.error(explain(err, 'hoàn tác')))
                .finally(() => refresh(wsId))
            },
          },
        })
        return saved
      } catch (err) {
        toast.error(explain(err, 'tạo sự kiện'))
        return null
      }
    },
    [wsId],
  )

  const remove = useCallback(
    (o: Occurrence, scope?: EditScope) => {
      const key = instanceKey(o)
      pendingRemovals.add(key)
      const snapshot = patchLists(wsId, (l) => removeInstances(l, o, scope))
      let undone = false
      const timer = setTimeout(() => {
        if (undone) return
        calendarApi
          .deleteEvent(wsId, o.eventId, scope ? { scope, occurrence: o.occurrenceStart } : {})
          .catch((err) => {
            restore(snapshot)
            toast.error(explain(err, 'xoá sự kiện'))
          })
          .finally(() => {
            pendingRemovals.delete(key)
            refresh(wsId)
          })
      }, DELETE_UNDO_MS)
      toast('Đã xoá sự kiện', {
        duration: DELETE_UNDO_MS,
        action: {
          label: 'Hoàn tác',
          onClick: () => {
            undone = true
            clearTimeout(timer)
            pendingRemovals.delete(key)
            restore(snapshot)
            refresh(wsId)
          },
        },
      })
    },
    [wsId],
  )

  return useMemo(() => ({ move, create, remove }), [move, create, remove])
}

/** Sends a move back. After a series change the instance sits at its new start, and `following` lives on the tail event. */
async function undoMove(wsId: string, o: Occurrence, saved: EventDetail, scope: EditScope | undefined, movedTo: string) {
  try {
    const back = { start: o.start, end: o.end, allDay: o.allDay, version: saved.version }
    if (!scope) await calendarApi.updateEvent(wsId, o.eventId, back)
    else if (scope === 'this') await calendarApi.updateEvent(wsId, saved.eventId, back, { scope, occurrence: o.occurrenceStart })
    else await calendarApi.updateEvent(wsId, saved.eventId, back, { scope: 'all', occurrence: movedTo })
  } catch (err) {
    toast.error(explain(err, 'hoàn tác'))
  } finally {
    refresh(wsId)
  }
}

export interface RsvpVars {
  occurrence: Occurrence
  response: RsvpInput['response']
  scope: 'this' | 'all'
  comment?: string
}

/** Answers an invitation: the block changes at once and goes back if the server refuses. */
export function useRsvp(wsId: string) {
  return useMutation({
    mutationFn: ({ occurrence: o, response, scope, comment }: RsvpVars) =>
      calendarApi.rsvp(wsId, o.eventId, { response, scope, comment, occurrence: scope === 'this' ? o.occurrenceStart : undefined }),
    onMutate: async ({ occurrence: o, response, scope }) => {
      await queryClient.cancelQueries({ queryKey: keys.calendar.eventsAll(wsId) })
      return patchLists(wsId, (l) => answerInstances(l, o, response, scope))
    },
    onError: (err, _vars, snapshot) => {
      if (snapshot) restore(snapshot)
      toast.error(explain(err, 'trả lời lời mời'))
    },
    onSettled: () => refresh(wsId),
  })
}

export function useUpdateSubscription(wsId: string) {
  return useMutation({
    mutationFn: ({ calId, patch }: { calId: string; patch: SubscriptionPatch }) =>
      calendarApi.updateSubscription(wsId, calId, patch),
    onMutate: async ({ calId, patch }) => {
      const key = keys.calendar.calendars(wsId)
      await queryClient.cancelQueries({ queryKey: key })
      const before = queryClient.getQueryData<{ calendars: CalendarSummary[] }>(key)
      if (before) {
        queryClient.setQueryData(key, {
          ...before,
          calendars: before.calendars.map((c) => (c.id === calId ? { ...c, ...patch } : c)),
        })
      }
      return before
    },
    onError: (err, _vars, before) => {
      if (before) queryClient.setQueryData(keys.calendar.calendars(wsId), before)
      toast.error(explain(err, 'đổi lịch'))
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: keys.calendar.calendars(wsId) }),
  })
}
