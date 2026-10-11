import { useState, type CSSProperties, type ReactNode } from 'react'
import { Pin } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useLongPress } from '../../hooks/useLongPress'
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
  /** Open the message's thread; absent inside the thread panel itself. */
  onReply?: (id: string) => void
  /** The "Thêm" button was pressed; the menu hangs from `anchor`. */
  onMore?: (m: Message, anchor: HTMLElement) => void
  /** Long press on a touch screen, or the Menu key: open the action sheet. */
  onOpenSheet?: (m: Message) => void
}

interface MessageBlockProps extends MessageActions {
  message: Message & { _optimistic?: boolean }
  people: PeopleDirectory
  /** Ids for the containing article's accessible name. */
  idBase: string
  /** Your own message: right side, no avatar, no name. */
  mine?: boolean
  /** Follows a message of the same sender within five minutes: no avatar or name. */
  grouped?: boolean
  /** Realtime: flash the author's avatar. */
  halo?: boolean
  /** Realtime label after the time ("vừa gửi", "vừa trả lời"), in the author's hue. */
  tag?: string
  tagStyle?: CSSProperties
  className?: string
  /** Its action sheet is open: the bubble is ringed in the accent. */
  pressed?: boolean
  /** Under the bubble, on the message's side: the thread summary row. */
  children?: ReactNode
}

/**
 * One message in the stream (DESIGN.md "Chat: luồng in-line"): others on the
 * left with avatar and name, yours on the right with neither, the text in a
 * bubble, and attachments, reactions and the thread summary outside it on the
 * same side. Shared by the main stream and the thread panel.
 */
export function MessageBlock({
  message: m, people, idBase, mine = false, grouped = false, pressed = false, halo, tag, tagStyle, className = '', children,
  me, onReact, onToggleReaction, onPin, onReply, onMore, onOpenSheet,
}: MessageBlockProps) {
  const name = displayName(people, m.sender_id, m.sender_name)
  const avatarUrl = people.byUserId.get(m.sender_id)?.avatarUrl
  const file = m.linked_entity_type === 'drive_file' && m.linked_entity_id
    ? m.content.replace(/<[^>]+>/g, '').match(/^📎\s+(.+)$/)?.[1]?.trim()
    : undefined
  const pending = !!m._optimistic
  const [timeShown, setTimeShown] = useState(false)
  const press = useLongPress(() => onOpenSheet?.(m), !!onOpenSheet && !pending)

  const time = (
    <time
      className="text-xs text-ink-muted tnum shrink-0"
      title={formatDateTime(m.created_at)}
      dateTime={typeof m.created_at === 'string' ? m.created_at : undefined}
    >
      {pending ? 'Đang gửi' : formatTime(m.created_at)}
    </time>
  )
  const pinned = m.is_pinned && (
    <span className="inline-flex items-center gap-1 text-xs text-ink-muted shrink-0">
      <Pin size={12} strokeWidth={1.75} aria-hidden="true" /> Đã ghim
    </span>
  )

  return (
    <div
      {...press}
      onClick={(e) => {
        // A tap on a grouped line shows its time where hover cannot; taps on controls are theirs.
        if (grouped && !(e.target as HTMLElement).closest('a, button, [role="button"]')) setTimeShown((v) => !v)
      }}
      onKeyDown={(e) => {
        if (onOpenSheet && !pending && (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey))) {
          e.preventDefault()
          onOpenSheet(m)
        }
      }}
      className={`group relative grid gap-x-2 md:gap-x-3 px-1 md:px-3.5 rounded-surface pointer-coarse:select-none
        ${mine ? 'grid-cols-[minmax(0,1fr)_2rem]' : 'grid-cols-[2rem_minmax(0,1fr)]'}
        ${grouped ? 'pt-0.5' : 'pt-2'} ${pending ? 'opacity-70' : ''} ${className}`}
    >
      {!mine && (grouped
        ? <GutterTime shown={timeShown} className="col-start-1 row-start-1">{time}</GutterTime>
        : <Avatar name={name} hueKey={m.sender_id || name} src={avatarUrl} size={32} halo={halo} className="col-start-1 row-start-1" />)}
      {mine && grouped && <GutterTime shown={timeShown} className="col-start-2 row-start-1">{time}</GutterTime>}

      <div className={`min-w-0 flex flex-col row-start-1 ${mine ? 'col-start-1 items-end' : 'col-start-2 items-start'}`}>
        {grouped ? (
          <>
            <span id={`${idBase}-who`} className="sr-only">{mine ? 'Bạn' : name}</span>
            {pinned}
          </>
        ) : (
          <div className={`flex items-baseline gap-2 min-w-0 max-w-full ${mine ? 'justify-end' : ''}`}>
            {mine
              ? <span id={`${idBase}-who`} className="sr-only">Bạn</span>
              : <span id={`${idBase}-who`} className="font-semibold text-ink truncate">{name}</span>}
            {time}
            {pinned}
            {tag && (
              <span className="rt-tag text-xs font-semibold shrink-0" style={tagStyle}>{tag}</span>
            )}
          </div>
        )}

        {file ? (
          <div id={`${idBase}-text`} className="mt-0.5 max-w-full md:max-w-[min(70ch,85%)]">
            {isImageFile(file)
              ? <ImagePreviewCard fileId={m.linked_entity_id!} filename={file} />
              : <FilePreviewCard fileId={m.linked_entity_id!} filename={file} />}
          </div>
        ) : (
          <div
            id={`${idBase}-text`}
            className={`mt-0.5 w-fit max-w-[88%] md:max-w-[min(70ch,85%)] px-3 py-1.75 rounded-xl wrap-anywhere
              [&_code]:bg-base [&_pre]:bg-base
              ${mine ? 'bg-accent-wash' : 'bg-sunk'} ${pressed ? 'ring-2 ring-accent' : ''}
              ${grouped ? '' : mine ? 'rounded-tr-sm' : 'rounded-tl-sm'}`}
          >
            <MessageContent content={m.content} contentFormat={m.content_format} />
          </div>
        )}

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
          mine={mine}
          onReact={() => onReact(m.id)}
          onReply={onReply ? () => onReply(m.id) : undefined}
          onPin={() => onPin(m)}
          isPinned={m.is_pinned}
          onMore={onMore ? (anchor) => onMore(m, anchor) : undefined}
        />
      )}
    </div>
  )
}

/** The time of a grouped line, in the avatar column: shown on hover, focus or tap. */
function GutterTime({ shown, className, children }: { shown: boolean; className: string; children: ReactNode }) {
  return (
    <span
      className={`${className} pt-1 text-center opacity-0 transition-opacity duration-quick
        group-hover:opacity-100 group-focus-within:opacity-100 ${shown ? 'opacity-100' : ''}`}
    >
      {children}
    </span>
  )
}
