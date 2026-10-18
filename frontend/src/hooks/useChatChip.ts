import { useCallback, useSyncExternalStore } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { readChatChip, subscribeChatChip, writeChatChip } from '../lib/chat-chip'
import { chatTabOf, validateChannelsSearch, type ChatTab } from '../lib/nav-destinations'
import { useAuthStore } from '../stores/auth.store'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useModuleSettings } from './useModuleSettings'

/**
 * The chip of the phone Chat list that is showing, and the way to change it. The address wins (`?tab=`,
 * so a link or a back step lands on its chip). With none, Chat opens where Cài đặt Chat → "Mở Chat ở" says:
 * on Mục when the person chose it, otherwise on the chip they last chose on this device (Tất cả on a new
 * one). Choosing replaces the address entry rather than adding one: a chip is a view of the same screen,
 * not a place to go back to.
 */
export function useChatChip() {
  const userId = useAuthStore((s) => s.user?.id ?? '')
  const navigate = useNavigate()
  const search = validateChannelsSearch(useSearch({ strict: false }) as Record<string, unknown>)
  const remembered = useSyncExternalStore(subscribeChatChip, () => readChatChip(userId), () => 'all' as ChatTab)
  const { open_at } = useModuleSettings('chat', useActiveWorkspace().workspaceId)
  const opening: ChatTab = open_at === 'sections' ? 'sections' : remembered

  const select = useCallback((chip: ChatTab) => {
    writeChatChip(userId, chip)
    // Tất cả is the address without a chip, unless Chat opens on Mục: then it has to be named.
    const tab = chip === 'all' && open_at !== 'sections' ? undefined : chip
    void navigate({ to: '/channels', search: { ...search, tab }, replace: true })
  }, [userId, navigate, search, open_at])

  return { chip: chatTabOf(search, opening), select }
}
