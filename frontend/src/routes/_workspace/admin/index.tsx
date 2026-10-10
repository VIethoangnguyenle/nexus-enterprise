import { createFileRoute } from '@tanstack/react-router'
import { OverviewScreen } from '../../../components/admin/OverviewScreen'

export const Route = createFileRoute('/_workspace/admin/')({
  component: OverviewScreen,
})
