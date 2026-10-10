import { createFileRoute } from '@tanstack/react-router'
import { UsersScreen } from '../../../components/admin/UsersScreen'

export const Route = createFileRoute('/_workspace/admin/users')({
  component: UsersScreen,
})
