import { Avatar, Text } from '../primitives'
import { FilePreviewCard } from './FilePreviewCard'

interface MessageItemProps {
  senderName: string
  content: string
  replyCount?: number
  isGrouped?: boolean
  onReply?: () => void
  linkedEntityType?: string
  linkedEntityId?: string
}

/** Extract a display filename from message content that starts with 📎. */
function extractFilename(content: string): string | null {
  const match = content.match(/^📎\s+(.+)$/)
  return match ? match[1].trim() : null
}

/** Single message with avatar, sender, content, file attachment card, and reply action. */
export function MessageItem({ senderName, content, replyCount, isGrouped, onReply, linkedEntityType, linkedEntityId }: MessageItemProps) {
  const hasFileAttachment = linkedEntityType === 'drive_file' && linkedEntityId
  const attachedFilename = hasFileAttachment ? extractFilename(content) : null

  return (
    <div className={`group flex gap-2.5 px-4 py-1 hover:bg-bg-hover transition-colors
      ${isGrouped ? '' : 'mt-3'}`}>
      {/* Avatar column — only show on first message in group */}
      <div className="w-8 flex-shrink-0 pt-0.5">
        {!isGrouped && <Avatar name={senderName || '?'} size="md" />}
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        {!isGrouped && (
          <div className="flex items-baseline gap-2 mb-0.5">
            <span className="text-sm font-semibold text-text-primary">{senderName}</span>
          </div>
        )}

        {/* Text content — hide raw content if it's just the 📎 filename placeholder */}
        {attachedFilename ? null : (
          <p className="text-sm text-text-secondary leading-relaxed m-0 break-words">{content}</p>
        )}

        {/* File attachment card */}
        {hasFileAttachment && (
          <FilePreviewCard
            fileId={linkedEntityId!}
            filename={attachedFilename || 'File'}
          />
        )}

        {/* Reply action */}
        {onReply && (
          <button
            onClick={onReply}
            className="mt-1 px-2 py-0.5 text-xs text-text-muted hover:text-accent-hover
              hover:bg-bg-active rounded border-none bg-transparent cursor-pointer
              transition-colors opacity-0 group-hover:opacity-100"
          >
            💬 {replyCount ? `${replyCount} replies` : 'Reply'}
          </button>
        )}
      </div>
    </div>
  )
}
