import { useNavigate } from '@tanstack/react-router'
import { CalendarX, Video } from 'lucide-react'
import type { Attendee, EventDetail, RsvpResponse } from '../../api/calendar'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCalendars, useEventDetail, useRsvp } from '../../hooks/useCalendar'
import { useJoinEventCall } from '../../hooks/useCalendarCall'
import { useNow } from '../../hooks/useNow'
import { callWindow } from '../../lib/calendar-call'
import { colorStyle } from '../../lib/calendar-color'
import { formatWhen, spanOf, toDateKey, weekdayShort } from '../../lib/calendar-format'
import { statusOf } from '../../lib/errors'
import { Avatar, Button, Pressable } from '../primitives'
import { CardSkeleton, UnavailableCard } from '../chat/ChatCard'

/** What a guest can answer from the card. */
const ANSWERS: { response: Exclude<RsvpResponse, 'needs_action'>; label: string }[] = [
  { response: 'accepted', label: 'Có' },
  { response: 'declined', label: 'Không' },
  { response: 'tentative', label: 'Có thể' },
]

const FACES = 3
const FRAME = 'grid gap-2.5 w-full max-w-110 px-3 py-3 rounded-lg bg-raised border border-line'

/** The title the message was posted with ("📅 Gặp đối tác VNPay"), for a card whose event is gone. */
const titleOfText = (text: string) => text.replace(/^📅\s*/u, '').trim() || 'Sự kiện'

/** "1 có · 1 chưa trả lời": who said yes and who has not answered; no guest list, no count. */
function summaryOf(d: EventDetail): string {
  if (d.guestCount === 0 && d.attendees.length === 0) return 'Chưa mời ai'
  if (d.attendees.length === 0) return `${d.guestCount} khách`
  const n = (r: RsvpResponse) => d.attendees.filter((a) => a.response === r).length
  return `${n('accepted')} có · ${n('needs_action')} chưa trả lời`
}

function Stack({ people }: { people: Attendee[] }) {
  return (
    <span className="flex" aria-hidden="true">
      {people.slice(0, FACES).map((a, i) => (
        <span key={a.person.nodeId} className={`rounded-full ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`}>
          <Avatar name={a.person.displayName} hue={a.person.hue} src={a.person.avatarUrl} size={24} />
        </span>
      ))}
    </span>
  )
}

interface Props {
  eventId: string
  /** The conversation the card is in: the event must be on this group's calendar. */
  channelId: string
  /** The text of the message, which names the event after it is gone. */
  text: string
}

/**
 * The event of a group's calendar, in the conversation (DESIGN.md "Lịch của nhóm chat", mockup §10): a date
 * box, the title, when and where, the guests and how many have answered, and the viewer's own answer as three
 * choices when they are a guest. Everything is read live from the calendar under the viewer's own access, so a
 * move shows at once; a cancelled event is dimmed and says so.
 */
export function EventChatCard({ eventId, channelId, text }: Props) {
  const { workspaceId: wsId } = useActiveWorkspace()
  const detail = useEventDetail(wsId, eventId)
  const calendars = useCalendars(wsId)
  const rsvp = useRsvp(wsId)
  const join = useJoinEventCall()
  const navigate = useNavigate()
  const now = useNow()

  if (detail.isError) {
    if (statusOf(detail.error) === 404) {
      const title = titleOfText(text)
      return (
        <div data-card role="group" aria-label={`Sự kiện đã huỷ: ${title}`} className={`${FRAME} opacity-60`}>
          <div className="flex items-center gap-3">
            <span className="grid place-items-center shrink-0 w-13 h-14 rounded-xl bg-sunk text-ink-muted">
              <CalendarX size={22} strokeWidth={1.75} aria-hidden="true" />
            </span>
            <span className="grid min-w-0">
              <b className="font-semibold text-ink-muted line-through break-words">{title}</b>
              <span className="text-xs font-semibold text-ink-muted">Đã huỷ</span>
            </span>
          </div>
        </div>
      )
    }
    return <UnavailableCard tone="accent" icon={<CalendarX size={16} strokeWidth={1.75} aria-hidden="true" />} title="Không xem được sự kiện này" />
  }
  const d = detail.data
  if (!d || calendars.isPending) return <CardSkeleton />

  // A card names its event by id; one that points at an event outside this group is not shown as its own.
  const calendar = calendars.data?.find((c) => c.id === d.calendarId)
  if (calendar && (calendar.kind !== 'channel' || calendar.channel?.id !== channelId)) {
    return <UnavailableCard tone="accent" icon={<CalendarX size={16} strokeWidth={1.75} aria-hidden="true" />} title="Không xem được sự kiện này" />
  }

  const { start, end } = spanOf(d)
  const window = d.hasCall && !d.allDay ? callWindow(start, end, now) : 'over'
  const running = window === 'open' && now.getTime() >= start.getTime()
  const answers = !d.isOrganizer && d.myResponse !== undefined
  const where = [formatWhen(start, end, d.allDay), d.location].filter(Boolean).join(' · ')

  return (
    <div data-card role="group" aria-label={`Sự kiện: ${d.title}`} className={FRAME} style={colorStyle(calendar?.color ?? 3)}>
      <div className="grid grid-cols-[3.25rem_minmax(0,1fr)] gap-3 items-center">
        <span className="grid place-items-center content-center w-13 h-14 rounded-xl bg-(--w) text-(--c) leading-none" aria-hidden="true">
          <small className="text-2xs font-semibold tracking-wide">{d.allDay ? 'CẢ NGÀY' : weekdayShort(start)}</small>
          <b className="font-display text-2xl font-bold text-ink">{start.getDate()}</b>
        </span>
        <span className="grid gap-0.5 min-w-0">
          <b className="font-semibold text-ink break-words">{d.title}</b>
          <span className="text-xs text-ink-muted tnum break-words">{where}</span>
          <span className="flex items-center gap-1.5 min-h-6">
            <Stack people={d.attendees} />
            <span className="text-xs text-ink-muted">{summaryOf(d)}</span>
          </span>
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {answers && (
          <span role="radiogroup" aria-label="Trả lời" className="inline-flex p-0.5 rounded-full bg-sunk">
            {ANSWERS.map((a) => {
              const on = d.myResponse === a.response
              return (
                <Pressable
                  key={a.response}
                  role="radio"
                  aria-checked={on}
                  disabled={rsvp.isPending}
                  onClick={() => !on && rsvp.mutate({ occurrence: d, response: a.response, scope: 'all' })}
                  className={`h-8 min-w-11 px-3 rounded-full text-sm font-semibold text-center
                    ${on ? 'bg-success-wash text-success' : 'text-ink hover:bg-hover'}`}
                >
                  {a.label}
                </Pressable>
              )
            })}
          </span>
        )}
        {window === 'open' && (
          <Button variant="success" size="sm" className="rounded-full" loading={join.isPending} onClick={() => join.mutate({ eventId: d.eventId, channelId })}>
            <Video size={16} strokeWidth={1.75} aria-hidden="true" />
            Tham gia
          </Button>
        )}
        {running && <span className="text-xs font-semibold text-success">Đang diễn ra</span>}
        <span className="grow" />
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void navigate({ to: '/calendar', search: (prev: { ws?: string }) => ({ ws: prev.ws, date: toDateKey(start) }) })}
        >
          Mở trong Lịch
        </Button>
      </div>
    </div>
  )
}
