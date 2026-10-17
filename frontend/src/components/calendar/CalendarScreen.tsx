import { useCallback, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { addDays, isSameDay, setHours, startOfDay } from 'date-fns'
import { Plus } from 'lucide-react'
import type { EditScope, Occurrence, RsvpResponse } from '../../api/calendar'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import {
  useCalendarActions, useCalendars, useOccurrences, useRsvp, useUpdateSubscription,
} from '../../hooks/useCalendar'
import { useNow } from '../../hooks/useNow'
import { useBottomBarLayout, usePhone } from '../../hooks/usePhone'
import { useRealtimeWash } from '../../hooks/useRealtimeWash'
import { instanceKey } from '../../lib/calendar-cache'
import {
  AGENDA_DAYS, CALENDAR_VIEWS, parseDateKey, rangeOf, shiftDate, toDateKey, visibleDays, type CalendarView,
} from '../../lib/calendar-format'
import { validateCalendarSearch, type CalendarSearch } from '../../lib/calendar-search'
import { useAuthStore } from '../../stores/auth.store'
import { Button, IconButton } from '../primitives'
import { AgendaList } from './AgendaList'
import { CalendarList } from './CalendarList'
import { CalendarEmpty, CalendarError, GridSkeleton } from './CalendarStates'
import { CalendarTopBar } from './CalendarTopBar'
import { EventPopover } from './EventPopover'
import type { BlockRealtime } from './EventBlock'
import { MiniMonth } from './MiniMonth'
import { MonthGrid } from './MonthGrid'
import { QuickCreate, type QuickDraft } from './QuickCreate'
import { RecurrenceScopeDialog } from './RecurrenceScopeDialog'
import { TimeGrid } from './TimeGrid'
import { useCalendarShortcuts } from './useCalendarShortcuts'

const viewStorageKey = (userId: string | undefined) => `nexus.calendar.view.${userId ?? ''}`

/** The view this person left on this device, if it is one we still offer. */
function rememberedView(userId: string | undefined): CalendarView | undefined {
  try {
    const v = localStorage.getItem(viewStorageKey(userId))
    return (CALENDAR_VIEWS as readonly string[]).includes(v ?? '') ? (v as CalendarView) : undefined
  } catch {
    return undefined
  }
}

type Pending = { kind: 'move'; occurrence: Occurrence; start: Date; end: Date } | { kind: 'delete'; occurrence: Occurrence }

const EMPTY_HINT: Record<CalendarView, string> = {
  day: 'Hôm nay bạn chưa có sự kiện nào.',
  '3day': 'Ba ngày này bạn chưa có sự kiện nào.',
  week: 'Tuần này bạn chưa có sự kiện nào.',
  month: 'Tháng này bạn chưa có sự kiện nào.',
  agenda: 'Bạn chưa có sự kiện nào trong 30 ngày tới.',
}

/** Words that tell a change from the same event apart when the data comes back (not shown). */
const versionOf = (o: Occurrence) => `${instanceKey(o)}|${o.start}|${o.end}|${o.title}|${o.version}`

/**
 * Lịch (design/mockups/calendar.html): the Google-style top bar, the side column (Tạo, the small month, the
 * calendars) and the view. The address carries the view and the day, so reload, Back and a pasted link
 * land in the same place.
 */
export function CalendarScreen() {
  const { workspaceId: wsId } = useActiveWorkspace()
  const search = useSearch({ strict: false }) as CalendarSearch
  const navigate = useNavigate()
  const userId = useAuthStore((s) => s.user?.id)
  const now = useNow()
  const phone = usePhone()
  const compact = useBottomBarLayout()

  const view: CalendarView = search.mode ?? rememberedView(userId) ?? 'week'
  // A whole week does not fit a phone; it shows three days.
  const shownView: CalendarView = phone && view === 'week' ? '3day' : view
  const today = startOfDay(now)
  const date = parseDateKey(search.date) ?? today

  const go = useCallback(
    (next: { view?: CalendarView; date?: Date }) => {
      if (next.view) {
        try {
          localStorage.setItem(viewStorageKey(userId), next.view)
        } catch {
          // Not remembering is the only consequence.
        }
      }
      void navigate({
        to: '/calendar',
        search: (prev) => {
          // The previous address is read through the route's own rules, so only its own fields are carried on.
          const out: CalendarSearch = validateCalendarSearch({ ...prev })
          if (next.view) out.mode = next.view
          if (next.date) {
            if (isSameDay(next.date, new Date())) delete out.date
            else out.date = toDateKey(next.date)
          }
          return out
        },
      })
    },
    [navigate, userId],
  )

  // --- Data ---------------------------------------------------------------
  const calendarsQuery = useCalendars(wsId)
  const calendars = useMemo(() => calendarsQuery.data ?? [], [calendarsQuery.data])
  const byId = useMemo(() => new Map(calendars.map((c) => [c.id, c])), [calendars])
  const visibleIds = useMemo(() => calendars.filter((c) => c.visible).map((c) => c.id), [calendars])
  const writable = useMemo(() => calendars.filter((c) => c.access === 'write' || c.access === 'manage'), [calendars])

  const [agendaDays, setAgendaDays] = useState(AGENDA_DAYS)
  const range = useMemo(() => {
    const r = rangeOf(shownView, date)
    return shownView === 'agenda' ? { from: r.from, to: addDays(r.from, agendaDays) } : r
    // `date` is rebuilt from the address each render; its day is what matters.
  }, [shownView, toDateKey(date), agendaDays])
  const eventsQuery = useOccurrences(wsId, range.from, range.to, visibleIds)
  const occurrences = useMemo(() => eventsQuery.data ?? [], [eventsQuery.data])

  const colorOf = useCallback((o: Occurrence) => byId.get(o.calendarId)?.color ?? -1, [byId])

  const { fresh } = useRealtimeWash(occurrences, {
    idOf: (o) => o.eventId,
    versionOf,
    ready: eventsQuery.isSuccess && !eventsQuery.isPlaceholderData,
    scope: `${wsId}|${toDateKey(range.from)}|${toDateKey(range.to)}|${visibleIds.join(',')}`,
  })
  const realtime = useMemo(() => {
    const out = new Map<string, BlockRealtime>()
    for (const o of occurrences) {
      const a = fresh.get(versionOf(o))
      // Only a change somebody else made, and only when we know who: the wash is in their hue.
      if (a && a.source === 'other' && a.author) out.set(instanceKey(o), { author: a.author })
    }
    return out
  }, [fresh, occurrences])

  const actions = useCalendarActions(wsId)
  const rsvp = useRsvp(wsId)
  const subscription = useUpdateSubscription(wsId)

  // --- The event that is open ----------------------------------------------------
  const [opened, setOpened] = useState<{ eventId: string; occurrenceStart: string } | null>(null)
  const anchor = useRef<HTMLElement | null>(null)
  const openOccurrence = opened ? occurrences.find((o) => instanceKey(o) === instanceKey(opened)) ?? null : null

  const open = useCallback((o: Occurrence, el: HTMLElement) => {
    anchor.current = el
    setOpened({ eventId: o.eventId, occurrenceStart: o.occurrenceStart })
  }, [])
  const closeOpened = useCallback(() => {
    setOpened(null)
    anchor.current?.focus?.()
  }, [])

  // --- Quick create ---------------------------------------------------------------
  const [draft, setDraft] = useState<QuickDraft | null>(null)
  const draftRef = useRef<HTMLElement | null>(null)
  const createButton = useRef<HTMLButtonElement>(null)

  const startDraft = useCallback((next: QuickDraft) => {
    setOpened(null)
    setDraft(next)
  }, [])

  /** Tạo and the C key: an hour starting at the next hour (or 09:00 on another day), hanging from the button. */
  const createFromButton = useCallback(() => {
    const base = isSameDay(date, new Date()) ? new Date() : setHours(date, 8)
    const start = new Date(base)
    start.setHours(base.getHours() + 1, 0, 0, 0)
    draftRef.current = createButton.current
    startDraft({ start, end: new Date(start.getTime() + 3_600_000), allDay: false })
  }, [date, startDraft])

  // --- Scope question for a recurring change -----------------------------------------
  const [pending, setPending] = useState<Pending | null>(null)

  const handleMove = useCallback(
    (o: Occurrence, start: Date, end: Date) => {
      if (o.isRecurring) setPending({ kind: 'move', occurrence: o, start, end })
      else void actions.move({ occurrence: o, start, end })
    },
    [actions],
  )
  const handleDelete = useCallback(
    (o: Occurrence) => {
      setOpened(null)
      if (o.isRecurring) setPending({ kind: 'delete', occurrence: o })
      else actions.remove(o)
    },
    [actions],
  )
  const confirmScope = (scope: EditScope) => {
    const p = pending
    setPending(null)
    if (!p) return
    if (p.kind === 'move') void actions.move({ occurrence: p.occurrence, start: p.start, end: p.end, scope })
    else actions.remove(p.occurrence, scope)
  }

  const handleRsvp = (o: Occurrence, response: Exclude<RsvpResponse, 'needs_action'>, scope: 'this' | 'all') =>
    rsvp.mutate({ occurrence: o, response, scope })

  useCalendarShortcuts(
    {
      onView: (v) => go({ view: v }),
      onToday: () => go({ date: new Date() }),
      onStep: (steps) => go({ date: shiftDate(shownView, date, steps) }),
      onCreate: createFromButton,
      onDelete: openOccurrence?.canEdit ? () => handleDelete(openOccurrence) : undefined,
    },
    !pending,
  )

  // --- What the view shows ------------------------------------------------------------
  const columns = visibleDays(shownView, date).length
  const selectedKey = openOccurrence ? instanceKey(openOccurrence) : undefined
  const calendarFailed = calendarsQuery.isError
  const noneVisible = calendarsQuery.isSuccess && visibleIds.length === 0
  const loading = calendarsQuery.isPending || (visibleIds.length > 0 && eventsQuery.isPending && !eventsQuery.isError)

  let body
  if (calendarFailed) body = <CalendarError onRetry={() => void calendarsQuery.refetch()} />
  else if (loading) body = <GridSkeleton columns={shownView === 'month' || shownView === 'agenda' ? 7 : columns} />
  else if (noneVisible) {
    body = <CalendarEmpty text="Bạn đã ẩn mọi lịch. Chọn một lịch ở cột bên trái để xem sự kiện của nó." />
  } else if (eventsQuery.isError) body = <CalendarError onRetry={() => void eventsQuery.refetch()} />
  else if (shownView === 'month') {
    body = (
      <MonthGrid
        date={date}
        occurrences={occurrences}
        now={now}
        selectedKey={selectedKey}
        draftDay={draft?.allDay ? draft.start : null}
        draftRef={draftRef}
        realtime={realtime}
        keyOf={instanceKey}
        colorOf={colorOf}
        onOpen={open}
        onMove={handleMove}
        onOpenDay={(d) => go({ view: 'day', date: d })}
        onSelectDay={(d) => startDraft({ start: d, end: addDays(d, 1), allDay: true })}
      />
    )
  } else if (shownView === 'agenda') {
    body = occurrences.length === 0
      ? <CalendarEmpty text={EMPTY_HINT.agenda} onCreate={createFromButton} />
      : (
        <AgendaList
          from={range.from}
          days={agendaDays}
          occurrences={occurrences}
          now={now}
          selectedKey={selectedKey}
          realtime={realtime}
          keyOf={instanceKey}
          colorOf={colorOf}
          onOpen={open}
          onLoadMore={() => setAgendaDays((n) => n + AGENDA_DAYS)}
        />
      )
  } else {
    body = (
      <TimeGrid
        days={visibleDays(shownView, date)}
        occurrences={occurrences}
        now={now}
        selectedKey={selectedKey}
        draft={draft && !draft.allDay ? draft : null}
        draftRef={draftRef}
        realtime={realtime}
        keyOf={instanceKey}
        colorOf={colorOf}
        onOpen={open}
        onMove={handleMove}
        onSelect={(start, end) => startDraft({ start, end, allDay: false })}
        onOpenDay={(d) => go({ view: 'day', date: d })}
      />
    )
  }

  const showEmptyHint = !loading && !noneVisible && !calendarFailed && !eventsQuery.isError && occurrences.length === 0
    && shownView !== 'agenda'

  return (
    <div className="flex flex-1 min-h-0 min-w-0 bg-base" data-testid="calendar-screen">
      {!compact && (
        <aside aria-label="Lịch" className="w-64 shrink-0 overflow-y-auto grid content-start gap-5 pt-3 pr-3 pb-4 pl-4">
          <Button
            ref={createButton}
            variant="secondary"
            className="rounded-xl h-12 pl-4 pr-5 justify-self-start bg-raised! shadow-overlay border-none!"
            onClick={createFromButton}
          >
            <Plus size={22} strokeWidth={1.75} className="text-accent" aria-hidden="true" />
            Tạo
          </Button>
          <MiniMonth date={date} now={now} onPick={(d) => go({ date: d })} />
          {calendarsQuery.isPending && <div className="skeleton h-24 rounded-md" aria-hidden="true" />}
          {calendarsQuery.isSuccess && (
            <CalendarList
              calendars={calendars}
              onToggle={(c, visible) => subscription.mutate({ calId: c.id, patch: { visible } })}
              onColor={(c, color) => subscription.mutate({ calId: c.id, patch: { color } })}
            />
          )}
        </aside>
      )}

      <section aria-label="Lưới lịch" className="flex-1 min-w-0 flex flex-col min-h-0">
        <CalendarTopBar
          view={shownView}
          date={date}
          onView={(v) => go({ view: v })}
          onToday={() => go({ date: new Date() })}
          onStep={(steps) => go({ date: shiftDate(shownView, date, steps) })}
        />
        {showEmptyHint && (
          <div className="flex items-center gap-3 px-5 pb-2 text-sm text-ink-muted">
            <span>{EMPTY_HINT[shownView]}</span>
            <Button variant="soft" size="sm" className="rounded-full" onClick={createFromButton}>Tạo sự kiện</Button>
          </div>
        )}
        {body}
      </section>

      {compact && (
        <IconButton
          variant="filled"
          size="lg"
          aria-label="Tạo sự kiện"
          onClick={createFromButton}
          className="fixed z-sticky bottom-6 right-4 size-14! rounded-xl!"
        >
          <Plus size={26} strokeWidth={1.75} />
        </IconButton>
      )}

      <EventPopover
        wsId={wsId}
        occurrence={openOccurrence}
        calendar={openOccurrence ? byId.get(openOccurrence.calendarId) : undefined}
        anchorRef={anchor}
        onClose={closeOpened}
        onDelete={handleDelete}
        onRsvp={handleRsvp}
      />
      <QuickCreate
        draft={draft}
        anchorRef={draftRef}
        calendars={writable}
        onCancel={() => setDraft(null)}
        onSave={async (input) => {
          const saved = await actions.create(input)
          if (saved) setDraft(null)
          return !!saved
        }}
      />
      <RecurrenceScopeDialog
        open={!!pending}
        title={pending?.kind === 'delete' ? 'Xoá sự kiện lặp' : 'Dời sự kiện lặp'}
        onClose={() => setPending(null)}
        onConfirm={confirmScope}
      />
    </div>
  )
}
