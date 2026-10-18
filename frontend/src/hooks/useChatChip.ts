import { useCallback, useSyncExternalStore } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { readChatChip, subscribeChatChip, writeChatChip } from '../lib/chat-chip'
import { chatTabOf, validateChannelsSearch, type ChatTab } from '../lib/nav-destinations'
import { useAuthStore } from '../stores/auth.store'

/**
 * The chip of the phone Chat list that is showing, and the way to change it. The address wins (`?tab=`,
 * so a link or a back step lands on its chip); with none, the chip this person last chose on this device,
 * Tất cả on a new one. Choosing replaces the address entry rather than adding one: a chip is a view of
 * the same screen, not a place to go back to.
 */
export function useChatChip() {
  const userId = useAuthStore((s) => s.user?.id ?? '')
  const navigate = useNavigate()
  const search = validateChannelsSearch(useSearch({ strict: false }) as Record<string, unknown>)
  const remembered = useSyncExternalStore(subscribeChatChip, () => readChatChip(userId), () => 'all' as ChatTab)

  const select = useCallback((chip: ChatTab) => {
    writeChatChip(userId, chip)
    void navigate({ to: '/channels', search: { ...search, tab: chip === 'all' ? undefined : chip }, replace: true })
  }, [userId, navigate, search])

  return { chip: chatTabOf(search, remembered), select }
}
