import { createFileRoute } from '@tanstack/react-router'
import { RolesScreen } from '../../../components/admin/RolesScreen'

export const Route = createFileRoute('/_workspace/admin/roles')({
  component: RolesScreen,
})
