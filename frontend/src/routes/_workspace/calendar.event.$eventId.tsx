import { createFileRoute } from '@tanstack/react-router'
import { EditEventEditor } from '../../components/calendar/EventEditorRoutes'
import { validateEditEventSearch } from '../../lib/calendar-search'

// The full editor on an existing event; `occurrence` names the instance of a recurring one.
export const Route = createFileRoute('/_workspace/calendar/event/$eventId')({
  validateSearch: validateEditEventSearch,
  component: function EditEvent() {
    const { eventId } = Route.useParams()
    return <EditEventEditor eventId={eventId} search={Route.useSearch()} />
  },
})
