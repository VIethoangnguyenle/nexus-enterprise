import { createFileRoute } from '@tanstack/react-router'
import { CalendarScreen } from '../../components/calendar/CalendarScreen'
import { validateCalendarSearch } from '../../lib/calendar-search'

// The view and the day are part of the URL, so reload, Back/Forward and links keep the place. `?ws=` is kept by
// the layout. Lịch lives in the shared shell: one sign-in guard, one WebSocket, one workspace switcher.
export const Route = createFileRoute('/_workspace/calendar/')({
  validateSearch: validateCalendarSearch,
  component: CalendarScreen,
})
