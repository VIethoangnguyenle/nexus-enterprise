import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Message } from '../../api/messaging'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useAttachmentUploads } from '../../hooks/useAttachmentUploads'
import { useMessageActions } from '../../hooks/useMessageActions'
import { useMarkRead, useUnreadCounts } from '../../hooks/useMessaging'
import { useMessages, useSendMessage, useTogglePin, useToggleReaction } from '../../hooks/useMessages'
import { useFollowThread, useLoadOlderMessages } from '../../hooks/useThreads'
import { usePeople } from '../../hooks/usePeople'
import { explain } from '../../lib/errors'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { LazyChatEditor } from '../chat/lazy-parts'
import { ConversationStream } from '../spaces/ConversationStream'
import { toast } from '../primitives'

/**
 * The conversation's own main stream, in the call's side panel, with its
 * composer under it. It is the same stream and composer as the conversation
 * screen, so a message sent here is a message in the group like any other.
 * Threads are not opened from here: a reply count links nowhere in the call.
 */
export function CallChat({ channelId, title }: { channelId: string; title: string }) {
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user)
  const messagesQ = useMessages(channelId)
  const unreadQ = useUnreadCounts()
  const send = useSendMessage(channelId)
  const toggleReaction = useToggleReaction(channelId)
  const togglePin = useTogglePin(channelId)
  const follow = useFollowThread(channelId)
  const older = useLoadOlderMessages(channelId)
  const markRead = useMarkRead(channelId)
  const lifecycle = useMessageActions(channelId, me?.id)
  const uploads = useAttachmentUploads({ workspaceId, channelId })
  const sendTyping = useWebSocketStore((s) => s.sendTyping)
  const sendSubscribe = useWebSocketStore((s) => s.sendSubscribe)
  const sendUnsubscribe = useWebSocketStore((s) => s.sendUnsubscribe)
  const [atEnd, setAtEnd] = useState(true)

  useEffect(() => {
    sendSubscribe(channelId)
    return () => sendUnsubscribe(channelId)
  }, [channelId, sendSubscribe, sendUnsubscribe])

  const messages = useMemo(
    () => [...(messagesQ.data?.messages ?? [])].reverse().filter((m) => !m.parent_message_id),
    [messagesQ.data],
  )
  const channelUnread = (unreadQ.data?.channels ?? []).find((c) => c.channel_id === channelId)

  // Reading it here counts as reading it: same rule as the conversation screen.
  const lastPersisted = [...messages].reverse().find(
    (m) => m.id && !(m as { _optimistic?: boolean })._optimistic && !m.id.startsWith('temp-'),
  )?.id
  useEffect(() => {
    if (!lastPersisted || !atEnd) return
    const timer = setTimeout(() => markRead.mutate(lastPersisted), 1000)
    return () => clearTimeout(timer)
    // markRead is a fresh object every render; the message id and position are the trigger.
  }, [lastPersisted, channelId, atEnd])

  const handleSend = useCallback(
    async (content: string, mentions: string[], attachments?: { drive_file_id: string; width?: number; height?: number }[]) => {
      if (!content.trim() && (!attachments || attachments.length === 0)) return
      try {
        await send.mutateAsync({ content, mentions, attachments })
      } catch (err) {
        toast.error(explain(err, 'gửi tin nhắn'))
        throw err
      }
    },
    [send],
  )

  return (
    <div className="cs-chat">
      <ConversationStream
        channelId={channelId}
        label={`Tin nhắn trong ${title}`}
        messages={messages}
        ready={!messagesQ.isLoading}
        hasMore={!!messagesQ.data?.has_more}
        loadingOlder={older.loading}
        olderFailed={older.failed}
        onLoadOlder={older.loadOlder}
        me={me?.id}
        people={people}
        unread={{
          ready: unreadQ.isSuccess || unreadQ.isError,
          lastReadId: channelUnread?.last_read_message_id ?? '',
          count: channelUnread?.unread_count ?? 0,
        }}
        onOpenThread={() => {}}
        onReact={() => {}}
        onToggleReaction={(id, emoji, hasReacted) => toggleReaction.mutate({ messageId: id, emoji, hasReacted })}
        onPin={(m: Message) => togglePin.mutate({ messageId: m.id, isPinned: !!m.is_pinned })}
        onFollow={(m, on) => follow.mutate({ rootId: m.id, follow: on })}
        onAtEndChange={setAtEnd}
        lifecycle={lifecycle}
      />
      <LazyChatEditor
        channelId={channelId}
        people={people}
        tone="base"
        placeholder={`Nhắn trong ${title}`}
        onSend={handleSend}
        onTyping={() => sendTyping(channelId)}
        uploads={uploads}
        isPending={send.isPending}
      />
    </div>
  )
}
