import { useState, useRef, useEffect } from 'react'
import { Send, MessageSquare } from 'lucide-react'
import { PeekPanel } from '../composites/PeekPanel'
import { ErrorState } from '../ErrorState'
import { MessageContent } from '../chat'
import { Avatar, IconButton, Spinner, Text } from '../primitives'
import { useThread } from '../../hooks/useMessaging'
import { messagingApi } from '../../api/messaging'
import { queryClient } from '../../lib/query-client'
import { formatTimestamp } from './MessageList'

/** Thread side panel with reply editor at bottom. */
export function ThreadPanel({ messageId, channelId, onClose }: { messageId: string; channelId: string; onClose: () => void }) {
  const { data, isLoading, error, refetch } = useThread(messageId)
  const replies = data?.messages || []
  const [replyText, setReplyText] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Auto-scroll to bottom when new replies arrive
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [replies.length])

  const handleSendReply = async () => {
    if (!replyText.trim() || sending) return
    setSending(true)
    try {
      await messagingApi.sendMessage(channelId, {
        content: replyText.trim(),
        content_format: 'plain',
        parent_message_id: messageId,
      })
      setReplyText('')
      // Refresh thread + update reply count on parent
      queryClient.invalidateQueries({ queryKey: ['thread', messageId] })
      queryClient.invalidateQueries({ queryKey: ['messages', channelId] })
      inputRef.current?.focus()
    } finally {
      setSending(false)
    }
  }

  return (
    <PeekPanel title="Thread" onClose={onClose} width={340}>
      {/* Reply count header */}
      <div className="px-4 py-2 border-b border-outline-variant/50 flex items-center gap-2">
        <MessageSquare size={14} className="text-on-surface-variant" />
        <Text variant="caption" muted>{replies.length} {replies.length === 1 ? 'reply' : 'replies'}</Text>
      </div>

      {/* Replies list */}
      <div className="flex-1 overflow-y-auto py-2">
        {error && !error.message?.includes('Not Found') && !error.message?.includes('404') ? (
          <ErrorState title="Failed to load thread" message={error.message} onRetry={() => refetch()} />
        ) : isLoading ? (
          <div className="flex justify-center py-6"><Spinner /></div>
        ) : replies.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 px-4 text-center">
            <MessageSquare size={24} className="text-on-surface-variant mb-2" />
            <Text variant="caption" muted>No replies yet</Text>
            <Text variant="caption" muted>Be the first to reply</Text>
          </div>
        ) : (
          replies.map((r, i) => (
            <div key={r.id || i} className="flex gap-2 px-4 py-2 hover:bg-surface-container/50 transition-colors">
              <Avatar name={r.sender_name || '?'} size="sm" />
              <div className="flex-1 min-w-0">
                <div className="flex items-baseline gap-2 mb-1">
                  <span className="text-caption-ui text-on-surface font-medium">{r.sender_name}</span>
                  <span className="text-micro text-on-surface-variant">
                    {formatTimestamp(r.created_at)}
                  </span>
                </div>
                <MessageContent content={r.content} contentFormat={r.content_format} />
              </div>
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>

      {/* Reply editor */}
      <div className="border-t border-outline-variant p-3 bg-surface-container-lowest">
        <div className="flex items-center gap-2">
          <input
            ref={inputRef}
            value={replyText}
            onChange={(e) => setReplyText(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSendReply() } }}
            placeholder="Reply..."
            className="flex-1 bg-surface-container border border-outline-variant/50 rounded-lg px-3 py-2
              text-sm text-on-surface placeholder:text-on-surface-variant outline-none
              focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all"
          />
          <IconButton
            onClick={handleSendReply}
            disabled={!replyText.trim() || sending}
            aria-label="Send reply"
            className="bg-primary text-on-primary hover:bg-primary/90 rounded-full shadow-sm
              disabled:opacity-30 disabled:cursor-not-allowed flex-shrink-0"
          >
            {sending ? <Spinner size="sm" /> : <Send size={16} />}
          </IconButton>
        </div>
      </div>
    </PeekPanel>
  )
}
