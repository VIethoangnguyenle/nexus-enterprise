import { createFileRoute } from '@tanstack/react-router'
import { NewEventEditor } from '../../components/calendar/EventEditorRoutes'
import { validateNewEventSearch } from '../../lib/calendar-search'

// The full editor on a new event. The span, a title already typed or an event to copy ride in the address.
export const Route = createFileRoute('/_workspace/calendar/new')({
  validateSearch: validateNewEventSearch,
  component: function NewEvent() {
    return <NewEventEditor search={Route.useSearch()} />
  },
})
