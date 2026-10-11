import { useCallback, useMemo } from 'react'
import { useCanGoBack, useNavigate, useRouter } from '@tanstack/react-router'
import type { FeedMessage } from '../api/shortcuts'
import type { Conversation } from '../lib/conversations'
import { openTarget } from '../lib/shortcut-feed'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useBottomBarLayout } from './usePhone'
import { useChatNavigation } from './useChatNavigation'

/** Where a feed item leads, as the `/channels/$channelId` search: its thread when it is a reply, and the message to light. */
export function feedSearch(item: FeedMessage): { thread?: string; message: string } {
  const t = openTarget(item)
  return t.rootId ? { thread: t.rootId, message: t.messageId } : { message: t.messageId }
}

/**
 * Back inside the app, or Trang chủ when this screen was the first one opened. The browser's own
 * history length would also count pages of other sites, and Quay lại could leave the app.
 */
export function useBackOrHome(): () => void {
  const router = useRouter()
  const navigate = useNavigate()
  const canGoBack = useCanGoBack()
  return useCallback(() => {
    if (canGoBack) router.history.back()
    else void navigate({ to: '/channels' })
  }, [canGoBack, router, navigate])
}

/**
 * What Được nhắc tới and Đã gắn sao need from the app around them: the
 * workspace, the conversation lists, and the ways out (open an item, open a
 * conversation, back, Trang chủ). Both screens fill the main pane; on a phone
 * they are full screens with Quay lại, like a conversation.
 */
export function useShortcutScreen() {
  const navigate = useNavigate()
  const { workspaceId } = useActiveWorkspace()
  const nav = useChatNavigation(workspaceId)
  const phone = useBottomBarLayout()

  const byId = useMemo(() => new Map(nav.conversations.map((c) => [c.id, c])), [nav.conversations])
  const conversationOf = useCallback((id: string) => byId.get(id), [byId])

  const open = useCallback(
    (item: FeedMessage) => void navigate({ to: '/channels/$channelId', params: { channelId: item.channel_id }, search: feedSearch(item) }),
    [navigate],
  )
  const openConversation = useCallback(
    (c: Conversation) => void navigate({ to: '/channels/$channelId', params: { channelId: c.id } }),
    [navigate],
  )
  const home = useCallback(() => void navigate({ to: '/channels' }), [navigate])
  const back = useBackOrHome()
  /** A real link for each card, so it opens in a new tab; the workspace travels with it. */
  const hrefOf = useCallback((item: FeedMessage) => {
    const q = new URLSearchParams()
    const s = feedSearch(item)
    if (s.thread) q.set('thread', s.thread)
    q.set('message', s.message)
    if (workspaceId) q.set('ws', workspaceId)
    return `/channels/${item.channel_id}?${q}`
  }, [workspaceId])

  return {
    workspaceId, phone, conversations: nav.conversations, starred: nav.arranged.starred,
    conversationOf, open, openConversation, home, back: phone ? back : undefined, hrefOf,
  }
}

/** Opens Đã gắn sao: the "Xem" of the toast that follows starring a message. */
export function useOpenStarred(): () => void {
  const navigate = useNavigate()
  return useCallback(() => void navigate({ to: '/channels/starred' }), [navigate])
}
