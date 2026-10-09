import type { CSSProperties, ReactNode } from 'react'
import { Pin } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { formatDateTime, formatTime } from '../../lib/format'
import { Avatar } from '../primitives'
import { MessageContent } from '../chat/MessageContent'
import { ReactionBar } from '../chat/ReactionBar'
import { HoverActionBar } from '../chat/HoverActionBar'
import { FilePreviewCard } from '../patterns/FilePreviewCard'
import { ImagePreviewCard, isImageFile } from '../patterns/ImagePreviewCard'

export interface MessageActions {
  me: string | undefined
  onReact?: (id: string) => void
  onToggleReaction?: (id: string, emoji: string, hasReacted: boolean) => void
  onPin?: (m: Message) => void
  onReply?: (id: string) => void
}

interface MessageBlockProps extends MessageActions {
  message: Message & { _optimistic?: boolean }
  people: PeopleDirectory
  /** Ids for the containing article's accessible name. */
  idBase: string
  /** Realtime: flash the author's avatar. */
  halo?: boolean
  /** Realtime label after the time ("vừa gửi", "vừa trả lời"), in the author's hue. */
  tag?: string
  tagStyle?: CSSProperties
  className?: string
  children?: ReactNode
}

/**
 * One message: avatar, name, time, body, attachment, reactions, and hover
 * actions. Shared by a topic's opening message and by thread replies.
 */
export function MessageBlock({
  message: m, people, idBase, halo, tag, tagStyle, className = '', children,
  me, onReact, onToggleReaction, onPin, onReply,
}: MessageBlockProps) {
  const name = displayName(people, m.sender_id, m.sender_name)
  const avatarUrl = people.byUserId.get(m.sender_id)?.avatarUrl
  const file = m.linked_entity_type === 'drive_file' && m.linked_entity_id
    ? m.content.replace(/<[^>]+>/g, '').match(/^📎\s+(.+)$/)?.[1]?.trim()
    : undefined
  const pending = !!m._optimistic

  return (
    <div
      className={`group relative grid grid-cols-[32px_minmax(0,1fr)] gap-3 px-3.5 py-2 rounded-surface
        ${pending ? 'opacity-70' : ''} ${className}`}
    >
      <Avatar name={name} hueKey={m.sender_id || name} src={avatarUrl} size={32} halo={halo} />
      <div className="min-w-0">
        <div className="flex items-baseline gap-2 min-w-0">
          <span id={`${idBase}-who`} className="font-semibold text-ink truncate">{name}</span>
          <time className="text-xs text-ink-muted tnum shrink-0" title={formatDateTime(m.created_at)}>
            {pending ? 'Đang gửi' : formatTime(m.created_at)}
          </time>
          {m.is_pinned && (
            <span className="inline-flex items-center gap-1 text-xs text-ink-muted shrink-0">
              <Pin size={12} strokeWidth={1.75} aria-hidden="true" /> Đã ghim
            </span>
          )}
          {tag && (
            <span className="rt-tag text-xs font-semibold shrink-0" style={tagStyle}>
              {tag}
            </span>
          )}
        </div>
        <div id={`${idBase}-text`} className="max-w-[70ch]">
          {file ? (
            isImageFile(file) ? (
              <ImagePreviewCard fileId={m.linked_entity_id!} filename={file} />
            ) : (
              <FilePreviewCard fileId={m.linked_entity_id!} filename={file} />
            )
          ) : (
            <MessageContent content={m.content} contentFormat={m.content_format} />
          )}
        </div>
        {onToggleReaction && onReact && (
          <ReactionBar
            reactions={m.reactions || []}
            currentUserId={me ?? ''}
            onToggle={(emoji) => {
              const hasReacted = !!m.reactions?.some((r) => r.emoji === emoji && r.user_ids?.includes(me ?? ''))
              onToggleReaction(m.id, emoji, hasReacted)
            }}
            onAddReaction={() => onReact(m.id)}
          />
        )}
        {children}
      </div>
      {!pending && onReact && onPin && (
        <HoverActionBar
          onReact={() => onReact(m.id)}
          onReply={onReply ? () => onReply(m.id) : undefined}
          onPin={() => onPin(m)}
          isPinned={m.is_pinned}
        />
      )}
    </div>
  )
}
