import { SpaceView } from '../../components/spaces/SpaceView'
import { Route } from './channels.$channelId'

/** Keyed by channel so tab, panel and realtime state start fresh in each conversation. */
export function ChannelRoute() {
  const { channelId } = Route.useParams()
  const { thread, message } = Route.useSearch()

  return (
    <div className="flex-1 flex min-w-0 min-h-0 relative">
      <SpaceView key={channelId} channelId={channelId} initialThreadId={thread} focusMessageId={message} />
    </div>
  )
}
