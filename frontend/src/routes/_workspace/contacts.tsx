import { createFileRoute } from '@tanstack/react-router'
import { ContactsScreen } from '../../components/contacts/ContactsScreen'
import { validateContactsSearch } from '../../lib/contacts-search'

// The category, the place inside it, the open profile and the search are part of the URL
// (`?cat=roles&role=…`, `?person=…`), so back, forward and links land where the person was.
// `?ws=` is kept by the layout.
export const Route = createFileRoute('/_workspace/contacts')({
  validateSearch: validateContactsSearch,
  component: ContactsScreen,
})
