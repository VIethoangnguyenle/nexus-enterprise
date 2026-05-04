import { useRef, useEffect, memo } from 'react'
import { Pin, MessageSquare } from 'lucide-react'
import { LoadingState } from '../LoadingState'
import { MessageContent, ReactionBar, HoverActionBar } from '../chat'
import { Avatar } from '../primitives'
import { FilePreviewCard } from '../patterns/FilePreviewCard'
import { ImagePreviewCard, isImageFile } from '../patterns/ImagePreviewCard'
import type { Message } from '../../api/messaging'

/** Parse protobuf Timestamp ({seconds, nanos}), ISO string, or unix number. */
export function formatTimestamp(ts: unknown): string {
  if (!ts) return ''
  let date: Date
  if (typeof ts === 'object' && ts !== null && 'seconds' in ts) {
    date = new Date((ts as { seconds: number }).seconds * 1000)
  } else if (typeof ts === 'string') {
    date = new Date(ts)
  } else if (typeof ts === 'number') {
    date = new Date(ts * 1000)
  } else {
    return ''
  }
  if (isNaN(date.getTime())) return ''
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function parseTimestamp(ts: unknown): number | null {
  if (!ts) return null
  if (typeof ts === 'object' && ts !== null && 'seconds' in ts) {
    return (ts as { seconds: number }).seconds * 1000
  }
  if (typeof ts === 'string') return new Date(ts).getTime()
  if (typeof ts === 'number') return ts * 1000
  return null
}

/** Check if two timestamps are more than `minutes` apart. */
export function isTimeDiffLarge(ts1: unknown, ts2: unknown, minutes: number): boolean {
  const a = parseTimestamp(ts1)
  const b = parseTimestamp(ts2)
  if (!a || !b) return true
  return Math.abs(b - a) > minutes * 60 * 1000
}

/** Check if a timestamp divider should appear between two messages (>30 min gap). */
function needsTimestampDivider(prev: Message | null, current: Message): boolean {
  if (!prev) return false
  const prevTime = parseTimestamp(prev.created_at)
  const currTime = parseTimestamp(current.created_at)
  if (!prevTime || !currTime) return false
  return currTime - prevTime > 30 * 60 * 1000
}

/** Centered date/time pill divider between message groups. */
function TimestampDivider({ timestamp }: { timestamp: unknown }) {
  const ms = parseTimestamp(timestamp)
  if (!ms) return null
  const d = new Date(ms)
  const today = new Date()
  const isToday = d.toDateString() === today.toDateString()
  const label = isToday
    ? 'TODAY, ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString([], { month: 'short', day: 'numeric' }).toUpperCase() + ', ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  return (
    <div className="flex items-center justify-center my-2">
      <span className="px-3 py-1 rounded-full bg-surface-container-high text-on-surface-variant font-label-caps text-label-caps text-[10px]">{label}</span>
    </div>
  )
}

/** Message row with directional bubbles — incoming left/white, outgoing right/blue. */
export const MessageRow = memo(function MessageRow({
  message: m,
  isGrouped,
  currentUserId,
  onReply,
  onReact,
  onPin,
  onToggleReaction,
}: {
  message: Message
  isGrouped: boolean
  currentUserId: string
  onReply: () => void
  onReact: () => void
  onPin: () => void
  onToggleReaction: (emoji: string) => void
}) {
  const hasFileAttachment = m.linked_entity_type === 'drive_file' && m.linked_entity_id
  const attachedFilename = hasFileAttachment ? m.content.match(/^📎\s+(.+)$/)?.[1]?.trim() : null
  const isOwn = m.sender_id === currentUserId

  return (
    <div className={`group relative flex px-4 md:px-6 transition-colors ${isOwn ? 'justify-end' : 'justify-start'} ${isGrouped ? '' : ''}`}>
      <div className={`flex items-start ${isGrouped ? 'gap-0' : 'gap-4'} max-w-[85%] ${isOwn ? 'flex-row-reverse' : 'flex-row'}`}>
        {/* Avatar — Stitch: w-10 h-10 rounded-full shadow-sm mt-1. Hidden when grouped. */}
        {!isGrouped && (
          <div className="w-10 flex-shrink-0 mt-1">
            <Avatar name={m.sender_name || '?'} size="md" />
          </div>
        )}
        {isGrouped && <div className="w-10 flex-shrink-0" />}

        <div className={`flex-1 min-w-0 flex flex-col ${isOwn ? 'items-end' : 'items-start'} gap-1`}>
          {/* Name + timestamp */}
          {!isGrouped && (
            <div className={`flex items-baseline gap-2 px-1 ${isOwn ? 'flex-row-reverse' : ''}`}>
              <span className="font-semibold text-sm text-on-surface">
                {isOwn ? 'You' : m.sender_name}
              </span>
              <span className="text-xs text-on-surface-variant">
                {formatTimestamp(m.created_at)}
              </span>
            </div>
          )}

          {/* Message bubble */}
          {attachedFilename ? null : (
            <div className={isOwn
              ? `bg-primary border-none rounded-2xl rounded-tr-sm p-4 text-sm leading-relaxed text-on-primary shadow-[0_2px_12px_-2px_rgba(37,99,235,0.2)] break-words [&_p]:text-on-primary [&_span]:text-on-primary [&_a]:text-on-primary [&_a]:underline [&_strong]:text-on-primary [&_em]:text-on-primary [&_code]:text-on-primary [&_code]:bg-white/15`
              : `bg-surface-bright border border-outline-variant/30 rounded-2xl rounded-tl-sm p-4 text-sm leading-relaxed text-on-surface shadow-[0_2px_8px_-2px_rgba(0,0,0,0.05)] break-words`}>
              <MessageContent content={m.content} contentFormat={m.content_format} />
            </div>
          )}

          {/* File attachment */}
          {hasFileAttachment && (
            isImageFile(attachedFilename || '') ? (
              <ImagePreviewCard fileId={m.linked_entity_id!} filename={attachedFilename || 'File'} />
            ) : (
              <FilePreviewCard fileId={m.linked_entity_id!} filename={attachedFilename || 'File'} />
            )
          )}

          {/* Pin indicator */}
          {m.is_pinned && (
            <div className="text-micro text-primary mt-1 flex items-center gap-1"><Pin size={10} /> Pinned</div>
          )}

          {/* Reactions */}
          <ReactionBar
            reactions={m.reactions || []}
            currentUserId={currentUserId}
            onToggle={onToggleReaction}
            onAddReaction={onReact}
          />

          {/* Reply count */}
          {m.reply_count ? (
            <button
              onClick={onReply}
              className="mt-1 px-2 py-1 text-caption-ui text-primary hover:text-primary-hover
                hover:bg-primary/5 rounded-sm border-none bg-transparent cursor-pointer
                transition-colors duration-fast focus-ring"
            >
              <MessageSquare size={11} className="inline mr-1" /> {m.reply_count} replies
            </button>
          ) : null}
        </div>
      </div>

      {/* Hover actions */}
      <HoverActionBar
        onReply={onReply}
        onReact={onReact}
        onPin={onPin}
        isPinned={m.is_pinned}
      />
    </div>
  )
})

/** Scrollable message list with auto-scroll and timestamp dividers. */
export function VirtualizedMessageList({
  messages,
  isLoading,
  currentUserId,
  onReply,
  onReact,
  onPin,
  onToggleReaction,
}: {
  messages: Message[]
  isLoading: boolean
  currentUserId: string
  onReply: (id: string) => void
  onReact: (id: string) => void
  onPin: (m: Message) => void
  onToggleReaction: (msgId: string, emoji: string, hasReacted: boolean) => void
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const prevCountRef = useRef(0)

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (messages.length > prevCountRef.current && messages.length > 0) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
    prevCountRef.current = messages.length
  }, [messages.length])

  // Scroll to bottom on initial load
  useEffect(() => {
    if (messages.length > 0) {
      bottomRef.current?.scrollIntoView()
    }
  }, [isLoading])

  if (isLoading) return <div className="flex-1"><LoadingState /></div>

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-8 text-center">
        <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-4">
          <MessageSquare size={28} className="text-primary" />
        </div>
        <h3 className="text-sm font-semibold text-on-surface mb-1">No messages yet</h3>
        <p className="text-xs text-on-surface-variant max-w-[240px]">
          Start the conversation by sending the first message below.
        </p>
      </div>
    )
  }

  return (
    <div ref={scrollRef} className="flex-1 overflow-y-auto p-4 md:p-6">
      <div className="flex flex-col w-full">
        {messages.map((m, idx) => {
          const prev = idx > 0 ? messages[idx - 1] : null
          const isGrouped = !!prev && prev.sender_id === m.sender_id && !isTimeDiffLarge(prev.created_at, m.created_at, 5)
          const showDivider = needsTimestampDivider(prev, m)
          return (
            <div key={m.id || idx} className={isGrouped && !showDivider ? 'mt-0.5' : idx > 0 ? 'mt-4' : ''}>
              {showDivider && <TimestampDivider timestamp={m.created_at} />}
              <MessageRow
                message={m}
                isGrouped={isGrouped && !showDivider}
                currentUserId={currentUserId}
                onReply={() => onReply(m.id)}
                onReact={() => onReact(m.id)}
                onPin={() => onPin(m)}
                onToggleReaction={(emoji) => {
                  const hasReacted = m.reactions?.some(r => r.emoji === emoji && r.user_ids?.includes(currentUserId)) || false
                  onToggleReaction(m.id, emoji, hasReacted)
                }}
              />
            </div>
          )
        })}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
