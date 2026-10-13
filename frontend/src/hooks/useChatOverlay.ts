import { useCallback } from 'react'
import { useNavigate, useRouter, useRouterState, useSearch } from '@tanstack/react-router'
import { chatOverlayOf, validateChannelsSearch, type ChannelsSearch, type ChatOverlay } from '../lib/nav-destinations'
import type { ThreadsChip } from '../components/spaces/PhoneThreadsScreen'

declare module '@tanstack/history' {
  interface HistoryState {
    /** This entry was pushed to show a full-screen layer over the Chat list. */
    chatOverlay?: boolean
  }
}

/**
 * The full-screen layers over the phone Chat list (Tìm trong chat, Thread) live in the
 * address of `/channels` (`?search=1`, `?threads=new|followed`), so the system back
 * gesture closes them instead of leaving Chat. Opening pushes an entry and marks it;
 * closing goes back when that mark is on the current entry, and otherwise (the page was
 * opened on the address) replaces it with the same address minus the layer.
 */
export function useChatOverlay() {
  const navigate = useNavigate()
  const router = useRouter()
  const search = validateChannelsSearch(useSearch({ strict: false }) as Record<string, unknown>)
  const pushed = useRouterState({ select: (s) => !!s.location.state?.chatOverlay })

  const open = useCallback((layer: Pick<ChannelsSearch, 'search' | 'threads'>) => {
    void navigate({ to: '/channels', search: { ...search, search: undefined, threads: undefined, ...layer }, state: { chatOverlay: true } })
  }, [navigate, search])

  const close = useCallback(() => {
    if (pushed) router.history.back()
    else void navigate({ to: '/channels', search: { ...search, search: undefined, threads: undefined }, replace: true })
  }, [pushed, router, navigate, search])

  return {
    overlay: chatOverlayOf(search) as ChatOverlay | null,
    openSearch: useCallback(() => open({ search: 1 }), [open]),
    openThreads: useCallback((chip: ThreadsChip) => open({ threads: chip === 'unread' ? 'new' : 'followed' }), [open]),
    close,
  }
}
