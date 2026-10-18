import { useCallback } from 'react'
import { useNavigate, useRouterState, useSearch } from '@tanstack/react-router'
import { CHAT_SEARCH_KEYS, filtersFromUrl, parseChatSearch, urlFromFilters, type ChatSearchFilters } from '../lib/chat-search'
import { validateChannelsSearch } from '../lib/nav-destinations'

/**
 * The words and filters of the phone's search screen, which live in the address of `/channels` next to
 * `?search=1` so back, forward and reload keep them. `commit` replaces the screen's entry (it is a refinement of
 * the same screen, not a new place) and keeps the mark that tells the screen's close to go back.
 */
export function useChatSearchAddress() {
  const navigate = useNavigate()
  const raw = useSearch({ strict: false }) as Record<string, unknown>
  const url = parseChatSearch(raw)
  const pushed = useRouterState({ select: (s) => !!s.location.state?.chatOverlay })

  const commit = useCallback((q: string, filters: ChatSearchFilters) => {
    const rest = { ...validateChannelsSearch(raw) } as Record<string, unknown>
    for (const key of CHAT_SEARCH_KEYS) delete rest[key]
    void navigate({
      to: '/channels',
      search: { ...rest, search: 1, ...urlFromFilters(q, filters) },
      replace: true,
      state: pushed ? { chatOverlay: true } : undefined,
    })
    // `raw` is a new object on every render; what matters is what it holds.
  }, [navigate, pushed, JSON.stringify(raw)])

  return { q: url.q ?? '', filters: filtersFromUrl(url), commit }
}
