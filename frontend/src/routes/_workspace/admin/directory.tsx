import { createFileRoute } from '@tanstack/react-router'
import { DirectoryScopeScreen } from '../../../components/admin/DirectoryScopeScreen'

export const Route = createFileRoute('/_workspace/admin/directory')({
  component: DirectoryScopeScreen,
})
