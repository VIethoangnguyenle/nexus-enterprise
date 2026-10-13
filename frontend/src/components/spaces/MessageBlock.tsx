import { useState, useRef, type CSSProperties, type ReactNode } from 'react'
import { Pin, Star } from 'lucide-react'
import type { Message, MessageAttachment } from '../../api/messaging'
import { useLongPress } from '../../hooks/useLongPress'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { formatDateTime, formatTime } from '../../lib/format'
import { Avatar } from '../primitives'
import { MessageContent } from '../chat/MessageContent'
import { ReactionBar } from '../chat/ReactionBar'
import { HoverActionBar } from '../chat/HoverActionBar'
import { QuoteBlock } from './QuoteBlock'
import { AttachmentGrid } from '../chat/AttachmentGrid'
import { MessageCard } from '../chat/MessageCard'
import { cardKindOf } from '../../lib/message-model'
import { FileViewer, type ViewerFile } from '../files/FileViewer'

export interface MessageActions {
  me: string | undefined
  onReact?: (id: string) => void
  onToggleReaction?: (id: string, emoji: string, hasReacted: boolean) => void
  onPin?: (m: Message) => void
  /** Open the message's thread; absent inside the thread panel itself. */
  onReply?: (id: string) => void
  /** Your own text message: the pencil of the hover bar opens it for editing. */
  onEdit?: (m: Message) => void
  /** The "Thêm" button was pressed; the menu hangs from `anchor`. */
  onMore?: (m: Message, anchor: HTMLElement) => void
  /** Long press on a touch screen, or the Menu key: open the action sheet. */
  onOpenSheet?: (m: Message) => void
  /** The quote was pressed: go to the message it quotes. Absent when that message is not loaded. */
  onOpenQuote?: (quotedId: string) => void
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
  /** The message is being edited where it stands: this replaces the bubble. */
  editor?: ReactNode
  /** Under the bubble, on the message's side: the thread summary row. */
  children?: ReactNode
}

/**
 * One message in the stream (DESIGN.md "Chat: luồng in-line"): others on the
 * left with avatar and name, yours on the right with neither, the text in a
 * bubble, and attachments, reactions and the thread summary outside it on the
 * same side. A quote sits inside the bubble, "đã sửa" ends it, and a deleted
 * root that keeps its thread is a quiet "Tin nhắn đã bị xoá" frame without
 * files or reactions. Shared by the main stream and the thread panel.
 */
export function MessageBlock({
  message: m, people, idBase, mine = false, grouped = false, pressed = false, halo, tag, tagStyle, className = '', editor, children,
  me, onReact, onToggleReaction, onPin, onReply, onEdit, onMore, onOpenSheet, onOpenQuote,
}: MessageBlockProps) {
  const name = displayName(people, m.sender_id, m.sender_name)
  const avatarUrl = people.byUserId.get(m.sender_id)?.avatarUrl
  const deleted = !!m.deleted
  const [viewer, setViewer] = useState<{ files: ViewerFile[]; index: number } | null>(null)
  const openerRef = useRef<HTMLElement | null>(null)

  const legacyFilename = !deleted && m.linked_entity_type === 'drive_file' && m.linked_entity_id
    ? m.content.replace(/<[^>]+>/g, '').match(/^📎\s+(.+)$/)?.[1]?.trim()
    : undefined

  const attachments: MessageAttachment[] = m.attachments && m.attachments.length > 0
    ? m.attachments
    : legacyFilename && m.linked_entity_id
      ? [{
          drive_file_id: m.linked_entity_id,
          file_name: legacyFilename,
          mime_type: '',
          size_bytes: 0,
        }]
      : []

  // A poll, task or call message carries a fallback line of text for clients without cards; the card replaces it.
  const card = deleted ? undefined : cardKindOf(m)
  const isCallCard = card === 'call'
  const hasText = !deleted && !card?.match(/^(poll|task|call)$/) && !!m.content && !legacyFilename && m.content.trim() !== '' && m.content.trim() !== '<p></p>'
  const hasBubble = hasText || (!deleted && !!m.quoted)
  const pending = !!m._optimistic
  const [timeShown, setTimeShown] = useState(false)
  const press = useLongPress(() => onOpenSheet?.(m), !!onOpenSheet && !pending && !deleted && !editor)

  const time = (
    <time
      className="text-xs text-ink-muted tnum shrink-0"
      title={formatDateTime(m.created_at)}
      dateTime={typeof m.created_at === 'string' ? m.created_at : undefined}
    >
      {pending ? 'Đang gửi' : formatTime(m.created_at)}
    </time>
  )
  const starred = !!m.starred && !deleted && (
    <span role="img" aria-label="Bạn đã gắn sao" title="Bạn đã gắn sao" className="inline-flex shrink-0 text-accent">
      <Star size={12} strokeWidth={1.75} fill="currentColor" aria-hidden="true" />
    </span>
  )
  const pinned = m.is_pinned && !deleted && (
    <span className="inline-flex items-center gap-1 text-xs text-ink-muted shrink-0">
      <Pin size={12} strokeWidth={1.75} aria-hidden="true" /> Đã ghim
    </span>
  )
  const showBar = !pending && !deleted && !editor && (onMore || onEdit || (onReact && onPin))

  return (
    <div
      {...press}
      onClick={(e) => {
        // A tap on a grouped line shows its time where hover cannot; taps on controls are theirs.
        if (grouped && !(e.target as HTMLElement).closest('a, button, [role="button"]')) setTimeShown((v) => !v)
      }}
      onKeyDown={(e) => {
        if (onOpenSheet && !pending && !deleted && !editor && (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey))) {
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
            {starred}
          </>
        ) : (
          <div className={`flex items-baseline gap-2 min-w-0 max-w-full ${mine ? 'justify-end' : ''}`}>
            {mine
              ? <span id={`${idBase}-who`} className="sr-only">Bạn</span>
              : <span id={`${idBase}-who`} className="text-md font-semibold text-ink truncate">{name}</span>}
            {time}
            {pinned}
            {starred}
            {tag && (
              <span className="rt-tag text-xs font-semibold shrink-0" style={tagStyle}>{tag}</span>
            )}
          </div>
        )}

        {deleted ? (
          <div
            id={`${idBase}-text`}
            className="mt-0.5 w-fit max-w-[88%] md:max-w-[min(70ch,85%)] px-3 py-1.75 rounded-xl text-16 md:text-md italic text-ink-muted
              ring-1 ring-inset ring-line"
          >
            Tin nhắn đã bị xoá
          </div>
        ) : editor ? (
          <div id={`${idBase}-text`} className="mt-0.5 w-full flex flex-col">
            <div className={`w-full flex ${mine ? 'justify-end' : ''}`}>{editor}</div>
          </div>
        ) : (
          <>
            {hasBubble && (
              <div
                id={`${idBase}-text`}
                className={`mt-0.5 w-fit max-w-[88%] md:max-w-[min(70ch,85%)] px-3 py-1.75 rounded-xl wrap-anywhere
                  [&_code]:bg-base [&_pre]:bg-base
                  ${mine ? 'bg-accent-wash' : 'bg-sunk'} ${pressed ? 'ring-2 ring-accent' : ''}
                  ${grouped ? '' : mine ? 'rounded-tr-sm' : 'rounded-tl-sm'}`}
              >
                {m.quoted && (
                  <QuoteBlock
                    quote={m.quoted}
                    people={people}
                    me={me}
                    onOpen={onOpenQuote && !m.quoted.deleted ? () => onOpenQuote(m.quoted!.id) : undefined}
                  />
                )}
                {hasText && (
                  <div className="flex flex-wrap items-end gap-x-1.5">
                    <div className="min-w-0 flex-auto">
                      <MessageContent content={m.content} contentFormat={m.content_format} />
                    </div>
                    {m.edited_at != null && (
                      <span title={`Sửa lúc ${formatDateTime(m.edited_at)}`} className="text-xs text-ink-muted whitespace-nowrap">đã sửa</span>
                    )}
                  </div>
                )}
              </div>
            )}
            {attachments.length > 0 && (
              <AttachmentGrid
                attachments={attachments}
                tone={mine ? 'raised' : 'base'}
                onOpen={(idx, files, opener) => {
                  openerRef.current = opener ?? null
                  setViewer({ files: files.map(viewerFileOf(name, m)), index: idx })
                }}
              />
            )}
            {card && (
              <div className="mt-1 w-full max-w-105 [&>*]:max-w-full">
                <MessageCard message={m} />
              </div>
            )}
          </>
        )}

        {!deleted && onToggleReaction && onReact && (
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
        {/* Call cards have no thread affordance (DESIGN.md §6, mockup §1 note 3). */}
        {!isCallCard && children}
      </div>

      {showBar && (
        <HoverActionBar
          mine={mine}
          onReact={onReact && onPin ? () => onReact(m.id) : undefined}
          onReply={onReply ? () => onReply(m.id) : undefined}
          onPin={onReact && onPin ? () => onPin(m) : undefined}
          isPinned={m.is_pinned}
          onEdit={onEdit ? () => onEdit(m) : undefined}
          onMore={onMore ? (anchor) => onMore(m, anchor) : undefined}
        />
      )}

      {viewer && (
        <FileViewer
          files={viewer.files}
          index={viewer.index}
          origin="chat"
          onClose={() => setViewer(null)}
          returnFocusRef={openerRef}
        />
      )}
    </div>
  )
}

/**
 * A message's file as the viewer reads it. The header names who sent it and when;
 * a message file carries no folder, so "Mở trong Tài liệu" is not offered.
 */
const viewerFileOf = (sender: string, m: Message) => (a: MessageAttachment): ViewerFile => ({
  id: a.drive_file_id,
  name: a.file_name,
  mime: a.mime_type || undefined,
  size: a.size_bytes || undefined,
  uploaderName: sender,
  uploaderId: m.sender_id,
  time: m.created_at,
})

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
