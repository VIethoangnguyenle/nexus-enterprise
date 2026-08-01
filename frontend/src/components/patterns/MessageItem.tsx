import { Avatar } from '../primitives'
import { FilePreviewCard } from './FilePreviewCard'
import { SmilePlus, Reply, MoreHorizontal, MessageSquare } from 'lucide-react'

interface MessageItemProps {
  senderName: string
  content: string
  timestamp?: string
  replyCount?: number
  isGrouped?: boolean
  isSelf?: boolean
  onReply?: () => void
  linkedEntityType?: string
  linkedEntityId?: string
}

/** Extract a display filename from message content that starts with 📎. */
function extractFilename(content: string): string | null {
  const match = content.match(/^📎\s+(.+)$/)
  return match ? match[1].trim() : null
}

/* --- Bubble style constants matching Stitch nexus-chat.html --- */

/** Incoming bubble: bg-surface-bright border border-outline-variant/30 rounded-2xl rounded-tl-sm p-4
 *  shadow-raised */
const BUBBLE_INCOMING = `bg-surface-bright border border-outline-variant/30
  rounded-2xl rounded-tl-sm p-4 text-on-surface text-sm leading-relaxed
  shadow-raised break-words`

/** Self bubble: bg-primary text-on-primary rounded-2xl rounded-tr-sm p-4
 *  shadow-accent-sm */
const BUBBLE_SELF = `bg-primary border-none
  rounded-2xl rounded-tr-sm p-4 text-on-primary text-sm leading-relaxed
  shadow-accent-sm break-words
  [&_p]:text-on-primary [&_span]:text-on-primary [&_a]:text-on-primary [&_a]:underline
  [&_strong]:text-on-primary [&_em]:text-on-primary
  [&_code]:text-on-primary [&_code]:bg-white/15`

/** Message item matching Stitch nexus-chat.html:
 *  Layout: flex items-start gap-4 max-w-[85%].
 *  Avatar: w-10 h-10 rounded-full shadow-sm mt-1.
 *  Name row: font-button text-body-md + font-body-sm text-body-sm for timestamp.
 *  Hover action bar: absolute -top-3 right-4, bg-surface-container-lowest, border-outline-variant/30. */
export function MessageItem({
  senderName,
  content,
  timestamp,
  replyCount,
  isGrouped,
  isSelf,
  onReply,
  linkedEntityType,
  linkedEntityId,
}: MessageItemProps) {
  const hasFileAttachment = linkedEntityType === 'drive_file' && linkedEntityId
  const attachedFilename = hasFileAttachment ? extractFilename(content) : null

  return (
    <div className={`flex items-start gap-4 max-w-[85%] animate-msg-slide-in
      ${isGrouped ? '' : 'mt-4'}
      ${isSelf ? 'self-end flex-row-reverse' : ''}`}>
      {/* Avatar — Stitch: w-10 h-10 rounded-full shadow-sm mt-1 */}
      <div className="w-10 shrink-0 mt-1">
        {!isGrouped && <Avatar name={senderName || '?'} size="md" />}
      </div>

      {/* Message content */}
      <div className={`flex flex-col ${isSelf ? 'items-end' : 'items-start'} gap-1`}>
        {/* Name + timestamp — Stitch: flex items-baseline gap-2 px-1 */}
        {!isGrouped && (
          <div className={`flex items-baseline gap-2 px-1 ${isSelf ? 'flex-row-reverse' : ''}`}>
            <span className="font-semibold text-sm text-on-surface">
              {isSelf ? 'You' : senderName}
            </span>
            {timestamp && (
              <span className="text-xs text-on-surface-variant">{timestamp}</span>
            )}
          </div>
        )}

        {/* Bubble — Stitch: relative group for hover actions */}
        <div className={`relative group ${isSelf ? BUBBLE_SELF : BUBBLE_INCOMING}`}>
          {/* Text content */}
          {attachedFilename ? null : (
            <p className="m-0 break-words message-html" dangerouslySetInnerHTML={{ __html: content }} />
          )}

          {/* File attachment card */}
          {hasFileAttachment && (
            <FilePreviewCard
              fileId={linkedEntityId!}
              filename={attachedFilename || 'File'}
            />
          )}

          {/* Hover action bar — Stitch: absolute -top-3 right-4 bg-surface-container-lowest
              border border-outline-variant/30 rounded-lg shadow-sm */}
          {/* eslint-disable no-restricted-syntax -- Segmented control: ba nút DÙNG CHUNG
              đường viền và chia nhau bo góc của hộp cha (rounded-l-lg, không bo, rounded-r-lg),
              phân cách bằng border-l. IconButton áp bo góc đồng đều cho cả bốn góc và hộp
              w-8 h-8 cố định, nên không diễn đạt được bo góc một phía lẫn viền dùng chung. */}
          <div className="absolute -top-3 right-4 bg-surface-container-lowest border border-outline-variant/30
            rounded-lg shadow-sm flex items-center opacity-0 group-hover:opacity-100 transition-opacity z-10">
            <button className="p-1.5 text-on-surface-variant hover:bg-surface-container hover:text-on-surface
              rounded-l-lg border-none bg-transparent cursor-pointer transition-colors">
              <SmilePlus size={18} />
            </button>
            <button className="p-1.5 text-on-surface-variant hover:bg-surface-container hover:text-on-surface
              border-l border-outline-variant/20 border-t-0 border-b-0 border-r-0 bg-transparent cursor-pointer transition-colors">
              <Reply size={18} />
            </button>
            <button className="p-1.5 text-on-surface-variant hover:bg-surface-container hover:text-on-surface
              border-l border-outline-variant/20 border-t-0 border-b-0 border-r-0 rounded-r-lg bg-transparent cursor-pointer transition-colors">
              <MoreHorizontal size={18} />
            </button>
          </div>
          {/* eslint-enable no-restricted-syntax */}
        </div>

        {/* Reply thread indicator */}
        {onReply && replyCount && replyCount > 0 && (
          /* eslint-disable-next-line no-restricted-syntax -- Nút dạng liên kết tô màu
             primary lúc nghỉ. Ghost của Button là text-on-surface-variant và không có trục
             tone như IconButton, nên màu sẽ đổi; hình học cũng lệch (px-2 py-1 rounded 4px
             so với px-3 py-1.5 rounded-md 8px). Một chỗ dùng, chưa đủ để thêm trục. */
          <button
            onClick={onReply}
            className="mt-1 px-2 py-1 text-xs text-primary hover:text-primary/80
              hover:bg-surface-container rounded border-none bg-transparent cursor-pointer
              transition-colors flex items-center gap-1"
          >
            <MessageSquare size={12} />
            {replyCount} {replyCount === 1 ? 'reply' : 'replies'}
          </button>
        )}
      </div>
    </div>
  )
}
