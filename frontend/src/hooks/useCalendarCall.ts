import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { calendarCallsApi } from '../api/calendar-calls'
import { callsApi } from '../api/calls'
import { callRoute } from '../lib/call-nav'
import { stashTicket } from '../stores/call.store'

/** The event whose call is joined; `channelId` is the group of a group calendar's event. */
export interface JoinEventCall {
  eventId: string
  channelId?: string
}

/**
 * Joins the video call of an event and opens the call screen with the ticket that came with it, the way the
 * conversation header does. The call of an event on a group's calendar is the group's own call (started or
 * joined like the one in the header); any other event has a call of its own. A refusal (not a guest, calls
 * off) is told by the mutation's toast.
 */
export function useJoinEventCall() {
  const navigate = useNavigate()
  return useMutation({
    meta: { action: 'tham gia cuộc gọi' },
    mutationFn: ({ eventId, channelId }: JoinEventCall) =>
      channelId ? callsApi.start(channelId, 'video') : calendarCallsApi.join(eventId),
    onSuccess: (ticket) => {
      stashTicket(ticket)
      void navigate(callRoute(ticket.call.id))
    },
  })
}
