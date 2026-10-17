import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { CalendarX } from 'lucide-react'
import type { CalendarSummary } from '../../api/calendar'
import { calendarsQueryOptions, eventQueryOptions } from '../../hooks/useCalendar'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useNow } from '../../hooks/useNow'
import { blankForm, defaultSpan, formFromDetail, guestOf, type EditorForm } from '../../lib/calendar-editor'
import { parseDateKey } from '../../lib/calendar-format'
import type { EditEventSearch, NewEventSearch } from '../../lib/calendar-search'
import { queryClient } from '../../lib/query-client'
import { statusOf } from '../../lib/errors'
import { keys } from '../../hooks/keys'
import { Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { CalendarError } from './CalendarStates'
import { EventEditor } from './EventEditor'
import { defaultCalendar } from './QuickCreate'

const writableOf = (calendars: CalendarSummary[]) => calendars.filter((c) => c.access === 'write' || c.access === 'manage')

function EditorSkeleton() {
  return (
    <div className="flex-1 grid content-start gap-4 p-5" aria-busy="true" aria-label="Đang tải trình soạn" data-testid="editor-skeleton">
      <div className="skeleton h-12 rounded-md" />
      <div className="skeleton h-9 w-2/3 rounded-md" />
      <div className="skeleton h-40 rounded-md" />
    </div>
  )
}

function Gone({ text }: { text: string }) {
  const navigate = useNavigate()
  return (
    <div className="flex-1 grid place-items-center">
      <EmptyState
        icon={<CalendarX size={24} strokeWidth={1.75} />}
        text={text}
        action={
          <Button variant="soft" className="rounded-full" onClick={() => void navigate({ to: '/calendar', search: (prev: { ws?: string }) => ({ ws: prev.ws }) })}>
            Về lịch
          </Button>
        }
      />
    </div>
  )
}

/** `/calendar/new`: a form on the span a click chose, on a title already typed, or on a copy of an event. */
export function NewEventEditor({ search }: { search: NewEventSearch }) {
  const { workspaceId: wsId } = useActiveWorkspace()
  const now = useNow()
  const calendarsQuery = useQuery(calendarsQueryOptions(wsId))
  const copy = useQuery({ ...eventQueryOptions(wsId, search.copy ?? ''), enabled: !!wsId && !!search.copy })
  const calendars = useMemo(() => calendarsQuery.data ?? [], [calendarsQuery.data])
  const writable = useMemo(() => writableOf(calendars), [calendars])

  const initial = useMemo<EditorForm | null>(() => {
    if (!calendarsQuery.isSuccess || (search.copy && !copy.isSuccess && !copy.isError)) return null
    const day = parseDateKey(search.date) ?? new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const chosen = writable.find((c) => c.id === search.calendar) ?? defaultCalendar(writable)
    const allDay = search.allDay === 1
    const at = (hhmm: string | undefined, fallback: Date) => {
      const m = hhmm?.match(/^(\d{2}):(\d{2})$/)
      if (!m) return fallback
      const d = new Date(day)
      d.setHours(Number(m[1]), Number(m[2]), 0, 0)
      return d
    }
    const fallback = defaultSpan(day, now)
    const start = at(search.start, fallback.start)
    let end = at(search.end, new Date(start.getTime() + 3_600_000))
    if (end <= start) end = new Date(start.getTime() + 3_600_000)
    if (copy.data) {
      const copied = formFromDetail(copy.data)
      const moved = blankForm({ start, end, allDay: copy.data.allDay, calendarId: chosen?.id ?? copy.data.calendarId })
      return { ...copied, calendarId: chosen?.id ?? copied.calendarId, startDate: moved.startDate, endDate: copy.data.allDay ? moved.endDate : moved.endDate, guests: [] }
    }
    const blank = blankForm({
      start: allDay ? day : start, end: allDay ? new Date(day.getTime() + 86_400_000) : end, allDay,
      calendarId: chosen?.id ?? '', title: search.title,
    })
    // An event made on a group's calendar is for the whole group.
    return chosen?.kind === 'channel' && chosen.channel
      ? { ...blank, inviteChannelId: chosen.channel.id, inviteChannelName: chosen.channel.name }
      : blank
  }, [calendarsQuery.isSuccess, copy.isSuccess, copy.isError, copy.data, search, writable, now])

  if (calendarsQuery.isError) return <CalendarError onRetry={() => void calendarsQuery.refetch()} />
  if (!initial) return <EditorSkeleton />
  if (writable.length === 0) return <Gone text="Bạn chưa có lịch nào để tạo sự kiện." />
  return <EventEditor key={`${search.copy ?? ''}`} wsId={wsId} initial={initial} calendars={writable} />
}

/** `/calendar/event/:id`: the event as the server has it. It is gone (404) or no longer the person's to change (403) is said in words. */
export function EditEventEditor({ eventId, search }: { eventId: string; search: EditEventSearch }) {
  const { workspaceId: wsId } = useActiveWorkspace()
  const calendarsQuery = useQuery(calendarsQueryOptions(wsId))
  const detail = useQuery(eventQueryOptions(wsId, eventId, search.occurrence))
  const [round, setRound] = useState(0)
  const calendars = useMemo(() => calendarsQuery.data ?? [], [calendarsQuery.data])

  const reload = () => {
    void queryClient.invalidateQueries({ queryKey: keys.calendar.event(wsId, eventId, search.occurrence) })
    setRound((n) => n + 1)
  }

  if (detail.isError) {
    const status = statusOf(detail.error)
    if (status === 404) return <Gone text="Sự kiện này không còn nữa." />
    if (status === 403) return <Gone text="Bạn không còn quyền xem sự kiện này." />
    return <CalendarError onRetry={() => void detail.refetch()} />
  }
  if (!detail.data || !calendarsQuery.isSuccess || detail.isFetching && round > 0) return <EditorSkeleton />
  const d = detail.data
  if (!d.canEdit) return <Gone text="Bạn không có quyền sửa sự kiện này." />

  // The server describes the instance that was named; the address's own start and end are only for a reply that is not about it.
  const describesInstance = !search.occurrence || d.occurrenceStart === search.occurrence
  const at = !describesInstance && search.start && search.end ? { start: search.start, end: search.end } : undefined
  const organizer = d.attendees.find((a) => a.isOrganizer)?.person ?? d.organizer
  // The calendar of the event stays offered even if it is not one the person can write to (a shared one they may only move events within).
  const own = calendars.find((c) => c.id === d.calendarId)
  const options = writableOf(calendars)
  const list = own && !options.includes(own) ? [own, ...options] : options
  return (
    <EventEditor
      key={`${d.eventId}:${d.version}:${round}`}
      wsId={wsId}
      initial={formFromDetail(d, at)}
      original={d}
      occurrence={search.occurrence}
      calendars={list}
      organizer={organizer ? guestOf(organizer) : undefined}
      onReload={reload}
    />
  )
}
