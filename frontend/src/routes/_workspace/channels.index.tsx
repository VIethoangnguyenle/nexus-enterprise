import { createFileRoute } from '@tanstack/react-router'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useChannels } from '../../hooks/useMessaging'
import { Navigate } from '@tanstack/react-router'
import { Spinner, Text } from '../../components/primitives'
import { MessageSquare } from 'lucide-react'

export const Route = createFileRoute('/_workspace/channels/')({
  component: ChannelsIndex,
})

/** Chat landing page — auto-selects the first available channel.
 *  Shows a welcome state when no channels exist. */
function ChannelsIndex() {
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = (wsParam && wsData?.workspaces?.find(w => w.id === wsParam)?.id)
    || wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading } = useChannels(wsId)
  const channels = data?.channels || []

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <Spinner size="lg" />
      </div>
    )
  }

  // Auto-navigate to first channel
  if (channels.length > 0) {
    return <Navigate to="/channels/$channelId" params={{ channelId: channels[0].id }} />
  }

  // No channels — empty state
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center px-4">
        <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-primary/10 flex items-center justify-center">
          <MessageSquare size={28} className="text-primary" />
        </div>
        <Text variant="body-strong" className="block mb-2">Welcome to Chat</Text>
        <Text variant="body" muted>Select a channel from the sidebar to start chatting.</Text>
      </div>
    </div>
  )
}
