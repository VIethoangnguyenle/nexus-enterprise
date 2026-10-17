import { useState, type RefObject } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  Check, CircleHelp, Copy, MapPin, MessageSquare, MoreHorizontal, Pencil, Repeat, Trash2, Users, Video, X,
} from 'lucide-react'
import type { Attendee, CalendarSummary, Occurrence, RsvpResponse } from '../../api/calendar'
import { useEventDetail } from '../../hooks/useCalendar'
import { useJoinEventCall } from '../../hooks/useCalendarCall'
import { useNow } from '../../hooks/useNow'
import { callWindow } from '../../lib/calendar-call'
import { colorTokens } from '../../lib/calendar-color'
import { formatWhen, responseWord, spanOf } from '../../lib/calendar-format'
import { Avatar, Button, Heading, IconButton, MenuItem, Popover, Pressable, Text } from '../primitives'

/** What a guest can answer. */
const ANSWERS: { response: Exclude<RsvpResponse, 'needs_action'>; label: string }[] = [
  { response: 'accepted', label: 'Có' },
  { response: 'declined', label: 'Không' },
  { response: 'tentative', label: 'Có thể' },
]

export interface EventBodyProps {
  wsId: string
  occurrence: Occurrence
  calendar?: CalendarSummary
  onClose: () => void
  /** Present only where an editor exists; the button is not drawn without it. */
  onEdit?: (o: Occurrence) => void
  onDuplicate?: (o: Occurrence) => void
  onDelete: (o: Occurrence) => void
  onRsvp: (o: Occurrence, response: Exclude<RsvpResponse, 'needs_action'>, scope: 'this' | 'all') => void
  /** The sheet on a phone draws bigger buttons. */
  large?: boolean
}

interface EventPopoverProps extends Omit<EventBodyProps, 'occurrence'> {
  /** The occurrence as the grid has it now (so an answer shows at once). Null closes the popover. */
  occurrence: Occurrence | null
  anchorRef: RefObject<HTMLElement | null>
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
        <Avatar name={a.person.displayName} hue={a.person.hue} src={a.person.avatarUrl} size={32} />
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
 * The details of an event (DESIGN.md §6, Popover chi tiết), the same in the card beside a block and in the
 * sheet of a phone. Guests answer here; the organizer sees "Bạn là người tổ chức". What the person may not do
 * is not drawn. The video call is the one primary action, and only from 15 minutes before the start until the end.
 */
export function EventDetailBody({ wsId, occurrence: o, calendar, onClose, onEdit, onDuplicate, onDelete, onRsvp, large }: EventBodyProps) {
  const detail = useEventDetail(wsId, o.eventId)
  const [scope, setScope] = useState<'this' | 'all'>('this')
  const [moreOpen, setMoreOpen] = useState(false)
  const navigate = useNavigate()
  const join = useJoinEventCall()
  const now = useNow()

  const { start, end } = spanOf(o)
  const attendees = detail.data?.attendees ?? []
  const asGuest = !o.isOrganizer && o.myResponse !== undefined
  const channel = calendar?.kind === 'channel' ? calendar.channel : undefined
  const hasMore = o.canEdit || !!channel
  const callState = o.hasCall && !o.allDay ? callWindow(start, end, now) : 'over'
  const iconSize = large ? 'size-11' : ''

  return (
    <div className="grid gap-1 pb-3">
      <div className="flex justify-end gap-0.5">
        {o.canEdit && onEdit && (
          <IconButton size="lg" shape="round" className={iconSize} aria-label="Sửa sự kiện" onClick={() => onEdit(o)}>
            <Pencil size={18} strokeWidth={1.75} />
          </IconButton>
        )}
        {o.canEdit && (
          <IconButton size="lg" shape="round" className={iconSize} aria-label="Xoá sự kiện" onClick={() => onDelete(o)}>
            <Trash2 size={18} strokeWidth={1.75} />
          </IconButton>
        )}
        {hasMore && (
          <IconButton
            size="lg" shape="round" className={iconSize} aria-label="Thêm" aria-expanded={moreOpen}
            onClick={() => setMoreOpen((v) => !v)}
          >
            <MoreHorizontal size={18} strokeWidth={1.75} />
          </IconButton>
        )}
        <IconButton size="lg" shape="round" className={iconSize} aria-label="Đóng" onClick={onClose}>
          <X size={18} strokeWidth={1.75} />
        </IconButton>
      </div>

      {moreOpen && hasMore && (
        <div role="group" aria-label="Tuỳ chọn khác" className="grid p-1 mx-2 mb-1 rounded-lg bg-sunk">
          {o.canEdit && onDuplicate && (
            <MenuItem icon={<Copy size={16} strokeWidth={1.75} />} onClick={() => onDuplicate(o)}>Nhân bản</MenuItem>
          )}
          {channel && (
            <MenuItem
              icon={<MessageSquare size={16} strokeWidth={1.75} />}
              onClick={() => void navigate({
                to: '/channels/$channelId', params: { channelId: channel.id }, search: (prev: { ws?: string }) => ({ ws: prev.ws }),
              })}
            >
              Mở cuộc trò chuyện
            </MenuItem>
          )}
          {o.canEdit && (
            <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={() => onDelete(o)}>Xoá</MenuItem>
          )}
        </div>
      )}

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
            <div className="grid gap-2 justify-items-start">
              <Text variant="small">Cuộc gọi video Nexus</Text>
              {callState === 'open' && (
                <Button
                  className={`rounded-full ${large ? 'h-11 px-5' : ''}`}
                  loading={join.isPending}
                  onClick={() => join.mutate({ eventId: o.eventId, channelId: channel?.id })}
                >
                  <Video size={16} strokeWidth={1.75} aria-hidden="true" />
                  Tham gia cuộc gọi
                </Button>
              )}
              {callState === 'early' && <Text variant="caption" muted>Mở từ 15 phút trước giờ bắt đầu.</Text>}
            </div>
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
              className={`${large ? 'h-10' : 'h-8'} px-3.5 rounded-full text-sm font-semibold
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
  )
}

/** The card beside a block (DESIGN.md §6, Popover chi tiết). Esc closes it and returns focus to the block. */
export function EventPopover({ occurrence: o, anchorRef, onClose, ...rest }: EventPopoverProps) {
  return (
    <Popover open={!!o} onClose={onClose} anchorRef={anchorRef} label={o?.title} className="w-105 max-w-screen">
      {o && <EventDetailBody occurrence={o} onClose={onClose} {...rest} />}
    </Popover>
  )
}
