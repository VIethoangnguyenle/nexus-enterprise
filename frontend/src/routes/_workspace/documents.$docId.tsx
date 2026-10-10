import { createFileRoute } from '@tanstack/react-router'
import { DocumentScreen } from '../../components/documents/DocumentScreen'
import { validateWorkspaceSearch } from '../../lib/workspace'

// One document. The list panel is not drawn here: the page needs the width.
export const Route = createFileRoute('/_workspace/documents/$docId')({
  validateSearch: validateWorkspaceSearch,
  component: DocumentScreen,
})
