import { Navigate, useNavigate, useSearch } from '@tanstack/react-router'
import { ChatSearchResults } from '../../components/spaces/ChatSearchResults'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { usePeople } from '../../hooks/usePeople'
import { useBackOrHome } from '../../hooks/useShortcutScreen'
import { filtersFromUrl, parseChatSearch, urlFromFilters } from '../../lib/chat-search'

/**
 * The results of Tìm trong chat in the content column of a desktop. Below 1024px the results live inside the
 * full-screen search (`/channels?search=1`), so the address is carried over to it.
 */
export function SearchRoute() {
  const navigate = useNavigate()
  const url = parseChatSearch(useSearch({ strict: false }) as Record<string, unknown>)
  const { workspaceId } = useActiveWorkspace()
  const nav = useChatNavigation(workspaceId)
  const people = usePeople(workspaceId)
  const phone = useBottomBarLayout()
  const back = useBackOrHome()

  if (phone) return <Navigate to="/channels" search={{ search: 1, ...url }} replace />

  const words = url.q ?? ''
  return (
    <ChatSearchResults
      variant="desktop"
      q={words}
      filters={filtersFromUrl(url)}
      // A filter is a refinement of this page, not a new place: it replaces the entry, so back still returns to the conversation.
      onFilters={(next) => void navigate({ to: '/channels/search', search: urlFromFilters(words, next), replace: true })}
      conversations={nav.conversations}
      people={people}
      onClose={() => {
        back()
        // Back to the conversation, and the focus back to the box.
        requestAnimationFrame(() => document.querySelector<HTMLElement>('[data-module-search]')?.focus())
      }}
    />
  )
}
