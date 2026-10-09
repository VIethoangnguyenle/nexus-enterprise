import { createFileRoute } from '@tanstack/react-router'
import { HomeView } from '../../components/spaces/HomeView'

export const Route = createFileRoute('/_workspace/channels/')({
  component: HomeView,
})
