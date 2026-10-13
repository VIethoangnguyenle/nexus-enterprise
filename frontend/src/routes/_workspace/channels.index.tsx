import { createFileRoute } from '@tanstack/react-router'
import { HomeView } from '../../components/spaces/HomeView'
import { PushPrompt } from '../../components/PushPrompt'
import { validateChannelsSearch } from '../../lib/nav-destinations'

function ChannelsIndex() {
  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0">
      <PushPrompt />
      <HomeView />
    </div>
  )
}

// `?tab=` is the phone's Chat tab (direct, sections, more; Trang chủ when absent). The desktop ignores it.
export const Route = createFileRoute('/_workspace/channels/')({
  validateSearch: validateChannelsSearch,
  component: ChannelsIndex,
})
