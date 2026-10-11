import { createFileRoute } from '@tanstack/react-router'
import { SpaceView } from '../../components/spaces/SpaceView'
import { validateWorkspaceSearch } from '../../lib/workspace'

export interface ChannelSearch {
  ws?: string
  thread?: string
}

export function validateChannelSearch(search: Record<string, unknown>): ChannelSearch {
  const out: ChannelSearch = { ...validateWorkspaceSearch(search) }
  if (typeof search.thread === 'string' && search.thread) {
    out.thread = search.thread
  }
  return out
}

export const Route = createFileRoute('/_workspace/channels/$channelId')({
  validateSearch: validateChannelSearch,
  component: ChannelRoute,
})

/** Keyed by channel so tab, panel and realtime state start fresh in each conversation. */
export function ChannelRoute() {
  const { channelId } = Route.useParams()
  // `search.thread` is validated and available for SpaceView to consume once it
  // gains an `initialThreadId` prop. Until then, ?thread= is validated but unused.
  // Do NOT render a separate ThreadPanel here (H3).

  return (
    <div className="flex-1 flex min-w-0 min-h-0 relative">
      <SpaceView key={channelId} channelId={channelId} />
    </div>
  )
}

