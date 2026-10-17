import { useState, type RefObject } from 'react'
import { Check, CircleHelp, MapPin, Pencil, Repeat, Trash2, Users, Video, X } from 'lucide-react'
import type { Attendee, CalendarSummary, Occurrence, RsvpResponse } from '../../api/calendar'
import { useEventDetail } from '../../hooks/useCalendar'
import { colorTokens } from '../../lib/calendar-color'
import { formatWhen, responseWord, spanOf } from '../../lib/calendar-format'
import { Avatar, Heading, IconButton, Popover, Pressable, Text } from '../primitives'

/** What a guest can answer. */
const ANSWERS: { response: Exclude<RsvpResponse, 'needs_action'>; label: string }[] = [
  { response: 'accepted', label: 'Có' },
  { response: 'declined', label: 'Không' },
  { response: 'tentative', label: 'Có thể' },
]

interface EventPopoverProps {
  wsId: string
  /** The occurrence as the grid has it now (so an answer shows at once). Null closes the popover. */
  occurrence: Occurrence | null
  calendar?: CalendarSummary
  anchorRef: RefObject<HTMLElement | null>
  onClose: () => void
  /** Present only where an editor exists; the button is not drawn without it. */
  onEdit?: (o: Occurrence) => void
  onDelete: (o: Occurrence) => void
  onRsvp: (o: Occurrence, response: Exclude<RsvpResponse, 'needs_action'>, scope: 'this' | 'all') => void
}

function Badge({ response }: { response: RsvpResponse }) {
  const tone = response === 'accepted' ? 'bg-success text-on-accent' : response === 'declined' ? 'bg-danger text-on-accent' : 'bg-raised text-ink-muted ring-1 ring-inset ring-line'
  return (
    <span aria-hidden="true" className={`absolute -right-0.75 -bottom-0.75 size-3.25 rounded-full grid place-items-center ring-2 ring-overlay ${tone}`}>
      {response === 'accepted' ? <Check size={9} strokeWidth={3} /> : response === 'declined' ? <X size={9} strokeWidth={3} /> : <CircleHelp size={9} strokeWidth={3} />}
    </span>
  )
}

function Guest({ a }: { a: Attendee }) {
  return (
    <li className="flex items-center gap-2.5">
      <span className="relative">
        <Avatar name={a.person.displayName} hueKey={a.person.nodeId} src={a.person.avatarUrl} size={32} />
        <Badge response={a.response} />
      </span>
      <span className="grid leading-tight min-w-0">
        <span className="truncate text-sm text-ink">{a.person.displayName}</span>
        <span className="text-xs text-ink-muted">
          {a.isOrganizer ? 'Người tổ chức' : responseWord(a.response)}
        </span>
      </span>
    </li>
  )
}

/** "2 đã nhận lời, 1 chưa trả lời", the one line over the guest list. */
function summaryOf(list: Attendee[]): string {
  const n = (r: RsvpResponse) => list.filter((a) => a.response === r).length
  const parts: [number, string][] = [
    [n('accepted'), 'đã nhận lời'], [n('declined'), 'đã từ chối'], [n('tentative'), 'chưa chắc'], [n('needs_action'), 'chưa trả lời'],
  ]
  return parts.filter(([count]) => count > 0).map(([count, word]) => `${count} ${word}`).join(', ')
}

const SHOWN_GUESTS = 8

/**
 * The details of an event in a card beside its block (DESIGN.md §6, Popover chi tiết). Esc closes it and
 * returns focus to the block. Guests answer here; the organizer sees "Bạn là người tổ chức". What the person
 * may not do is not drawn.
 */
export function EventPopover({ wsId, occurrence: o, calendar, anchorRef, onClose, onEdit, onDelete, onRsvp }: EventPopoverProps) {
  const detail = useEventDetail(wsId, o?.eventId)
  const [scope, setScope] = useState<'this' | 'all'>('this')
  const open = !!o

  const { start, end } = o ? spanOf(o) : { start: new Date(), end: new Date() }
  const attendees = detail.data?.attendees ?? []
  const asGuest = !!o && !o.isOrganizer && o.myResponse !== undefined

  return (
    <Popover open={open} onClose={onClose} anchorRef={anchorRef} label={o?.title} className="w-105 max-w-screen">
      {o && (
        <div className="grid gap-1 pb-3">
          <div className="flex justify-end gap-0.5">
            {o.canEdit && onEdit && (
              <IconButton size="lg" shape="round" aria-label="Sửa sự kiện" onClick={() => onEdit(o)}>
                <Pencil size={18} strokeWidth={1.75} />
              </IconButton>
            )}
            {o.canEdit && (
              <IconButton size="lg" shape="round" aria-label="Xoá sự kiện" onClick={() => onDelete(o)}>
                <Trash2 size={18} strokeWidth={1.75} />
              </IconButton>
            )}
            <IconButton size="lg" shape="round" aria-label="Đóng" onClick={onClose}>
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          </div>

          <div className="grid grid-cols-[2.5rem_1fr] items-start pr-3 gap-y-2">
            <span className="grid place-items-center h-6">
              <span aria-hidden="true" className="size-3.5 rounded-sm" style={{ background: colorTokens(calendar?.color ?? -1).fg }} />
            </span>
            <div className="grid gap-1">
              <Heading as="h2" className="text-xl! font-normal! font-body! leading-snug">{o.title}</Heading>
              <Text variant="small" muted>{formatWhen(start, end, o.allDay)}</Text>
              {o.recurrenceText && (
                <Text variant="small" muted className="flex items-center gap-1.5">
                  <Repeat size={13} strokeWidth={1.75} aria-hidden="true" />{o.recurrenceText}
                </Text>
              )}
            </div>

            {o.location && (
              <>
                <MapPin size={18} strokeWidth={1.75} className="text-ink-muted justify-self-center mt-0.5" aria-hidden="true" />
                <Text variant="small">{o.location}</Text>
              </>
            )}
            {o.hasCall && (
              <>
                <Video size={18} strokeWidth={1.75} className="text-ink-muted justify-self-center mt-0.5" aria-hidden="true" />
                <Text variant="small">Cuộc gọi video Nexus</Text>
              </>
            )}
            {detail.data?.description && (
              <>
                <span aria-hidden="true" />
                <Text variant="small" className="whitespace-pre-wrap">{detail.data.description}</Text>
              </>
            )}
            {calendar && (
              <>
                <span aria-hidden="true" />
                <Text variant="caption" muted>{calendar.name}</Text>
              </>
            )}
          </div>

          {detail.isError && (
            <Text variant="caption" muted className="px-4 pt-1">Chưa tải được chi tiết. Thông tin ở trên vẫn đúng.</Text>
          )}

          {(o.guestCount > 0 || attendees.length > 0) && (
            <div className="grid grid-cols-[2.5rem_1fr] items-start pr-3 pt-1">
              <Users size={18} strokeWidth={1.75} className="text-ink-muted justify-self-center mt-0.5" aria-hidden="true" />
              <div className="grid gap-2">
                <div className="grid leading-tight">
                  <span className="text-sm font-medium text-ink">{`${attendees.length || o.guestCount} khách`}</span>
                  {attendees.length > 0 && <span className="text-xs text-ink-muted">{summaryOf(attendees)}</span>}
                </div>
                {detail.isPending && o.guestCount > 0 && (
                  <div className="skeleton h-8 w-48 rounded-md" aria-hidden="true" />
                )}
                {attendees.length > 0 && (
                  <ul className="m-0 p-0 list-none grid gap-2">
                    {attendees.slice(0, SHOWN_GUESTS).map((a) => <Guest key={a.person.nodeId} a={a} />)}
                    {attendees.length > SHOWN_GUESTS && (
                      <li className="text-xs text-ink-muted">{`và ${attendees.length - SHOWN_GUESTS} người khác`}</li>
                    )}
                  </ul>
                )}
              </div>
            </div>
          )}

          {asGuest && (
            <div role="group" aria-label="Bạn sẽ tham gia?" className="flex flex-wrap items-center gap-2 mx-4 mt-1 pt-2.5 border-t border-line">
              <span className="text-sm text-ink mr-1">Bạn sẽ tham gia?</span>
              {ANSWERS.map((a) => (
                <Pressable
                  key={a.response}
                  aria-pressed={o.myResponse === a.response}
                  onClick={() => onRsvp(o, a.response, scope)}
                  className={`h-8 px-3.5 rounded-full text-sm font-semibold
                    ${o.myResponse === a.response ? 'bg-success-wash text-success' : 'ring-1 ring-inset ring-line text-ink hover:bg-hover'}`}
                >
                  {a.label}
                </Pressable>
              ))}
              {o.isRecurring && (
                <div role="group" aria-label="Phạm vi trả lời" className="flex gap-1.5 w-full pt-1">
                  {([['this', 'Chỉ lần này'], ['all', 'Cả chuỗi']] as const).map(([value, label]) => (
                    <Pressable
                      key={value}
                      aria-pressed={scope === value}
                      onClick={() => setScope(value)}
                      className={`h-7 px-3 rounded-full text-xs font-medium ${scope === value ? 'bg-accent-wash text-ink' : 'bg-sunk text-ink-muted hover:bg-hover'}`}
                    >
                      {label}
                    </Pressable>
                  ))}
                </div>
              )}
            </div>
          )}
          {o.isOrganizer && o.guestCount > 0 && (
            <Text variant="small" muted className="mx-4 mt-1 pt-2.5 border-t border-line">Bạn là người tổ chức</Text>
          )}
        </div>
      )}
    </Popover>
  )
}
