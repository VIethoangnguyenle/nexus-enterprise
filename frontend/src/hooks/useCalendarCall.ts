import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { calendarCallsApi } from '../api/calendar-calls'
import { callRoute } from '../lib/call-nav'
import { stashTicket } from '../stores/call.store'

/**
 * Joins the video call of an event and opens the call screen with the ticket that came with it, the way the
 * conversation header does. A refusal (not a guest, calls off) is told by the mutation's toast.
 */
export function useJoinEventCall() {
  const navigate = useNavigate()
  return useMutation({
    meta: { action: 'tham gia cuộc gọi' },
    mutationFn: (eventId: string) => calendarCallsApi.join(eventId),
    onSuccess: (ticket) => {
      stashTicket(ticket)
      void navigate(callRoute(ticket.call.id))
    },
  })
}
