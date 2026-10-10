import { createFileRoute } from '@tanstack/react-router'
import { ContactsScreen } from '../../components/contacts/ContactsScreen'

export const Route = createFileRoute('/_workspace/contacts')({
  component: ContactsScreen,
})
