import { createFileRoute } from '@tanstack/react-router'
import { ApprovalScreen } from '../../components/approval/ApprovalScreen'
import { validateApprovalSearch } from '../../lib/approval-search'

// The tab, the open request or template and the template builder are part of
// the URL, so reload, Back/Forward and links keep the place. `?ws=` is kept by
// the layout.
export const Route = createFileRoute('/_workspace/approval')({
  validateSearch: validateApprovalSearch,
  component: ApprovalScreen,
})
