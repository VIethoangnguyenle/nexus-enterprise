import { useState, useRef, type CSSProperties, type PointerEvent, type ReactNode } from 'react'
import { motion, useTransform } from 'motion/react'
import { MessageSquareReply, Pin, Reply, Star } from 'lucide-react'
import type { Message, MessageAttachment } from '../../api/messaging'
import { useLongPress } from '../../hooks/useLongPress'
import { useDoubleTap } from '../../hooks/useDoubleTap'
import { useMessageSwipe, MSG_SWIPE_SCALE } from '../../hooks/useMessageSwipe'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { EASE, useMotionPresets } from '../../lib/motion'
import { previewText } from '../../lib/chat-cache'
import { isImageAttachment } from '../../lib/file-kind'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { formatDateTime, formatTime } from '../../lib/format'
import { Avatar, Pressable } from '../primitives'
import { PersonLink } from '../primitives/PersonLink'
import { MessageContent } from '../chat/MessageContent'
import { ReactionBar } from '../chat/ReactionBar'
import { HoverActionBar } from '../chat/HoverActionBar'
import { QuoteBlock } from './QuoteBlock'
import { AttachmentGrid } from '../chat/AttachmentGrid'
import { MessageCard } from '../chat/MessageCard'
import { cardKindOf } from '../../lib/message-model'
import { FileViewer, type ViewerFile } from '../files/FileViewer'
import { ReactionSlider, type SlideRect } from './ReactionSlider'

export interface MessageActions {
  me: string | undefined
  onReact?: (id: string) => void
  onToggleReaction?: (id: string, emoji: string, hasReacted: boolean) => void
  onPin?: (m: Message) => void
  /** Open the message's thread; absent inside the thread panel itself. */
  onReply?: (id: string) => void
  /** Quote the message in the composer it is answered in: a swipe right on a phone, and its screen-reader action. */
  onQuote?: (m: Message) => void
  /**
   * The reaction a double-tap toggles on a phone (the person's Chat setting, 👍 by default); empty or absent: off.
   * Only where reactions are offered (`onReact` and `onToggleReaction`).
   */
  quickReaction?: string
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
  /**
   * The conversation's pictures in stream order: opening one of this message's pictures lets the viewer swipe
   * through all of them (mockup §9). Without it the viewer walks this message's files.
   */
  gallery?: ViewerFile[]
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
  gallery, me, onReact, onToggleReaction, onPin, onReply, onQuote, quickReaction, onEdit, onMore, onOpenSheet, onOpenQuote,
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

  // A poll, task, call or event message carries a fallback line of text for clients without cards; the card replaces it.
  const card = deleted ? undefined : cardKindOf(m)
  const isCallCard = card === 'call'
  const hasText = !deleted && !card?.match(/^(poll|task|call|calendar_event)$/) && !!m.content && !legacyFilename && m.content.trim() !== '' && m.content.trim() !== '<p></p>'
  const hasBubble = hasText || (!deleted && !!m.quoted)
  const pending = !!m._optimistic
  const [timeShown, setTimeShown] = useState(false)
  const live = !pending && !deleted && !editor

  // ---- Phone gestures (DESIGN.md §6 "Vuốt", mockup §9). None of them edits or deletes: that stays in the sheet.
  const phone = useBottomBarLayout()
  const { reduced } = useMotionPresets()
  const reactable = !deleted && !!onReact && !!onToggleReaction
  const reacted = (emoji: string) => !!m.reactions?.some((r) => r.emoji === emoji && r.user_ids?.includes(me ?? ''))
  const react = (emoji: string) => onToggleReaction?.(m.id, emoji, reacted(emoji))
  // A call card has no thread (DESIGN.md §6, mockup §1 note 3), so it has no swipe to one either.
  const threadAction = onReply && !isCallCard ? () => onReply(m.id) : undefined
  const quoteAction = onQuote ? () => onQuote(m) : undefined

  const swipeEnabled = phone && live && !!(quoteAction || threadAction)
  const swipe = useMessageSwipe({ onQuote: quoteAction, onThread: threadAction, enabled: swipeEnabled, reduced })
  const quoteScale = useTransform(swipe.rightProgress, [0, 0.999, 1], [0.6, 1, MSG_SWIPE_SCALE])
  const threadScale = useTransform(swipe.leftProgress, [0, 0.999, 1], [0.6, 1, MSG_SWIPE_SCALE])
  const quoteOpacity = useTransform(swipe.rightProgress, [0, 0.3, 1], [0, 0.5, 1])
  const threadOpacity = useTransform(swipe.leftProgress, [0, 0.3, 1], [0, 0.5, 1])
  const quoteColor = useTransform(swipe.rightProgress, (v) => (v >= 1 ? 'var(--color-accent)' : 'var(--color-ink-muted)'))

  // Long press: on a phone with reactions, the finger that pressed can slide onto the reaction row; lifting it
  // first opens the sheet as before. Elsewhere the long press opens the sheet at once.
  const rootRef = useRef<HTMLDivElement>(null)
  const bubbleRef = useRef<HTMLDivElement>(null)
  const finger = useRef<{ pointerId: number; x: number; y: number } | null>(null)
  const [slide, setSlide] = useState<{ start: { pointerId: number; x: number; y: number }; anchor: SlideRect; text: string } | null>(null)
  const slideable = phone && reactable && !!onOpenSheet
  const press = useLongPress(() => {
    const at = finger.current
    const el = bubbleRef.current ?? rootRef.current
    if (!slideable || !at || !el) {
      onOpenSheet?.(m)
      return
    }
    swipe.cancel()
    const r = el.getBoundingClientRect()
    setSlide({
      start: { ...at },
      anchor: { top: r.top, bottom: r.bottom, left: r.left, width: r.width },
      text: bubbleRef.current && hasText ? previewText(m.content) : '',
    })
  }, !!onOpenSheet && live)

  // Double-tap toggles the quick reaction; adding it pops the emoji over the bubble (360ms spring).
  const quick = reactable ? quickReaction ?? '' : ''
  const [burst, setBurst] = useState(0)
  const doubleTap = useDoubleTap(() => {
    const adding = !reacted(quick)
    react(quick)
    if (adding && !reduced) setBurst((n) => n + 1)
  }, phone && live && !!quick)

  const pointerHandlers = {
    onPointerDown: (e: PointerEvent<HTMLDivElement>) => {
      finger.current = { pointerId: e.pointerId, x: e.clientX, y: e.clientY }
      press.onPointerDown(e)
      doubleTap.onPointerDown?.(e)
    },
    onPointerMove: (e: PointerEvent<HTMLDivElement>) => {
      if (finger.current?.pointerId === e.pointerId) finger.current = { pointerId: e.pointerId, x: e.clientX, y: e.clientY }
      press.onPointerMove(e)
    },
    onPointerUp: (e: PointerEvent<HTMLDivElement>) => {
      press.onPointerUp()
      doubleTap.onPointerUp?.(e)
    },
    onPointerCancel: press.onPointerCancel,
    onPointerLeave: press.onPointerLeave,
    onContextMenu: press.onContextMenu,
  }

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

  const messageContent = (
    <div
      ref={rootRef}
      {...pointerHandlers}
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
        ${mine ? 'grid-cols-[minmax(0,1fr)] md:grid-cols-[minmax(0,1fr)_2rem]' : 'grid-cols-[2rem_minmax(0,1fr)]'}
        ${grouped ? 'pt-0.5' : 'pt-2'} ${pending ? 'opacity-70' : ''} ${className}`}
    >
      {!mine && (grouped
        ? <GutterTime shown={timeShown} className="col-start-1 row-start-1">{time}</GutterTime>
        : (
          <PersonLink userId={m.sender_id} people={people} name={name} avatarUrl={avatarUrl} secondary className="col-start-1 row-start-1 self-start rounded-full">
            <Avatar name={name} hueKey={m.sender_id || name} src={avatarUrl} size={32} halo={halo} />
          </PersonLink>
        ))}
      {/* Your side keeps an edge column for a grouped line's time only from md up; on a
          phone the bubble sits as close to the right edge as the avatar does on the left. */}
      {mine && grouped && <GutterTime shown={timeShown} className="max-md:hidden md:col-start-2 row-start-1">{time}</GutterTime>}

      <div className={`min-w-0 flex flex-col row-start-1 ${mine ? 'col-start-1 items-end' : 'col-start-2 items-start'}`}>
        {grouped ? (
          <>
            <span id={`${idBase}-who`} className="sr-only">{mine ? 'Bạn' : name}</span>
            {/* Below md there is no edge column on your side: a tap shows the time above the bubble. */}
            {mine && timeShown && <span className="md:hidden flex">{time}</span>}
            {pinned}
            {starred}
          </>
        ) : (
          <div className={`flex items-baseline gap-2 min-w-0 max-w-full ${mine ? 'justify-end' : ''}`}>
            {mine
              ? <span id={`${idBase}-who`} className="sr-only">Bạn</span>
              : (
                <PersonLink userId={m.sender_id} people={people} name={name} avatarUrl={avatarUrl}>
                  <span id={`${idBase}-who`} className="text-md font-semibold text-ink truncate">{name}</span>
                </PersonLink>
              )}
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
                ref={bubbleRef}
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
                  const opened = files[idx]
                  const inGallery = opened && gallery && isImageAttachment(opened)
                    ? gallery.findIndex((f) => f.id === opened.drive_file_id)
                    : -1
                  if (inGallery >= 0) setViewer({ files: gallery!, index: inGallery })
                  else setViewer({ files: files.map(viewerFileOf(name, m)), index: idx })
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

      {burst > 0 && (
        <span aria-hidden="true" className="absolute inset-0 grid place-items-center pointer-events-none z-10">
          <motion.span
            key={burst}
            className="text-[2.6rem] leading-none drop-shadow-lg"
            initial={{ scale: 0.4, opacity: 0 }}
            animate={{ scale: [0.4, 1.2, 1], opacity: [0, 1, 0] }}
            transition={{ duration: 0.36, ease: EASE.spring }}
            onAnimationComplete={() => setBurst(0)}
          >
            {quick}
          </motion.span>
        </span>
      )}

      {/* Each gesture as an action a screen reader can reach (DESIGN.md §6 "Vuốt": "Đọc màn hình"). */}
      {phone && live && (quoteAction || threadAction || onReact) && (
        <div className="sr-only" role="group" aria-label="Thao tác nhanh với tin">
          {quoteAction && <Pressable onClick={quoteAction}>Trích dẫn</Pressable>}
          {threadAction && <Pressable onClick={threadAction}>Trả lời trong thread</Pressable>}
          {onReact && <Pressable onClick={() => onReact(m.id)}>Thả cảm xúc</Pressable>}
        </div>
      )}

      {slide && (
        <ReactionSlider
          start={slide.start}
          anchor={slide.anchor}
          preview={slide.text ? (
            <div className={`flex ${mine ? 'justify-end' : ''}`}>
              <div className={`w-fit max-w-full px-3 py-1.75 rounded-xl wrap-anywhere text-16 shadow-overlay line-clamp-4
                ${mine ? 'bg-accent-wash' : 'bg-sunk'}`}>
                {slide.text}
              </div>
            </div>
          ) : undefined}
          onSelect={(emoji) => {
            setSlide(null)
            react(emoji)
          }}
          onLift={() => {
            setSlide(null)
            onOpenSheet?.(m)
          }}
          onCancel={() => setSlide(null)}
        />
      )}
    </div>
  )

  // Desktop and wide tablets keep the message as it was; a phone wraps it in the swipe with its two cues.
  if (!swipeEnabled) return messageContent

  return (
    <div className="relative overflow-x-clip" data-message-swipe>
      {quoteAction && (
        <motion.span
          aria-hidden="true"
          data-swipe-cue="quote"
          style={{ scale: quoteScale, opacity: quoteOpacity, color: quoteColor }}
          className="absolute inset-y-0 left-3 flex items-center pointer-events-none"
        >
          <Reply size={22} strokeWidth={1.75} />
        </motion.span>
      )}
      {threadAction && (
        <motion.span
          aria-hidden="true"
          data-swipe-cue="thread"
          style={{ scale: threadScale, opacity: threadOpacity }}
          className="absolute inset-y-0 right-3.5 flex items-center pointer-events-none"
        >
          <span className="grid place-items-center w-9 h-9 rounded-full bg-accent text-on-accent">
            <MessageSquareReply size={18} strokeWidth={1.75} />
          </span>
        </motion.span>
      )}
      <motion.div style={{ x: swipe.x }} className="relative touch-pan-y" {...swipe.bind}>
        {messageContent}
      </motion.div>
    </div>
  )
}

/**
 * A message's file as the viewer reads it. The header names who sent it and when;
 * a message file carries no folder, so "Mở trong Tài liệu" is not offered.
 */
export const viewerFileOf = (sender: string, m: Message) => (a: MessageAttachment): ViewerFile => ({
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
