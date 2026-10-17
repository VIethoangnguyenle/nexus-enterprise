import { apiFetch } from './client'
import type { CallJoin } from './calls'

/**
 * The video call of a calendar event. It is served by the messaging service (which owns calls), not the
 * calendar service, at `/api/calendar-events/:eventId/call`.
 */
export const calendarCallsApi = {
  /** Starts the event's call, or joins the one already open. 201 for a new call, 200 for an open one; both answer like StartCall. */
  join: (eventId: string) => apiFetch<CallJoin>(`/calendar-events/${eventId}/call`, { method: 'POST' }),
}
