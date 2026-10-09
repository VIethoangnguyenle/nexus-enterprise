import { createFileRoute } from '@tanstack/react-router'
import { SpaceView } from '../../components/spaces/SpaceView'

export const Route = createFileRoute('/_workspace/channels/$channelId')({ component: ChannelRoute })

/** Keyed by channel so tab, panel and realtime state start fresh in each conversation. */
function ChannelRoute() {
  const { channelId } = Route.useParams()
  return <SpaceView key={channelId} channelId={channelId} />
}
