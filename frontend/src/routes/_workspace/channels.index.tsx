import { createFileRoute } from '@tanstack/react-router'
import { HomeView } from '../../components/spaces/HomeView'
import { PushPrompt } from '../../components/PushPrompt'

function ChannelsIndex() {
  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0">
      <PushPrompt />
      <HomeView />
    </div>
  )
}

export const Route = createFileRoute('/_workspace/channels/')({
  component: ChannelsIndex,
})
