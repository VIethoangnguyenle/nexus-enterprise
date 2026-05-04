import { Avatar } from '../primitives'
import { Hash, MessageCircle } from 'lucide-react'

interface ChatListItemProps {
  id: string
  name: string
  channelType: string
  preview?: string
  timestamp?: string
  unreadCount?: number
  isActive?: boolean
  isStarred?: boolean
  isOnline?: boolean
  onClick: (id: string) => void
}

/** Chat list item matching Stitch nexus-chat.html:
 *  Active: bg-primary-fixed shadow-[0_2px_8px_-2px_rgba(0,0,0,0.05)] rounded-lg.
 *  Inactive: hover:bg-surface-container rounded-lg.
 *  Channel avatar: w-10 h-10 rounded-lg bg-surface-container-highest (groups) or rounded-full (DMs).
 *  Name: font-button text-body-md, timestamp: font-body-sm text-body-sm.
 *  Preview: font-body-md text-body-sm text-on-surface-variant truncate.
 *  Unread: w-2 h-2 bg-primary rounded-full absolute dot. */
export function ChatListItem({
  id,
  name,
  channelType,
  preview,
  timestamp,
  unreadCount,
  isActive,
  isOnline,
  onClick,
}: ChatListItemProps) {
  const isExternal = channelType === 'private'
  const isDM = channelType === 'dm'
  const hasUnread = unreadCount && unreadCount > 0

  return (
    <button
      className={`flex items-start gap-3 p-3 cursor-pointer border-none
        text-left w-full rounded-lg transition-colors relative group
        ${isActive
          ? 'bg-primary-fixed shadow-[0_2px_8px_-2px_rgba(0,0,0,0.05)]'
          : 'bg-transparent hover:bg-surface-container'
        }`}
      onClick={() => onClick(id)}
    >
      {/* Avatar — Stitch: w-10 h-10, rounded-lg for groups, rounded-full for DMs */}
      {isDM ? (
        <div className="relative flex-shrink-0">
          <Avatar name={name} size="md" />
          {isOnline && (
            <span className="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full bg-green-500 border-2 border-surface-container-lowest" />
          )}
        </div>
      ) : (
        <div className={`w-10 h-10 flex-shrink-0 flex items-center justify-center font-bold text-lg
          ${isActive
            ? 'rounded-lg bg-primary text-on-primary'
            : 'rounded-lg bg-surface-container-highest text-on-surface'
          }`}
        >
          {channelType === 'group' ? (
            <Hash size={20} />
          ) : (
            <MessageCircle size={20} />
          )}
        </div>
      )}

      {/* Text content */}
      <div className="flex-1 min-w-0">
        {/* Row 1: name + timestamp */}
        <div className="flex justify-between items-baseline mb-0.5">
          <div className="flex items-center gap-1.5 min-w-0 pr-2">
            <span className={`text-sm font-semibold truncate ${isActive ? '' : 'text-on-surface'}`}>
              {name}
            </span>
          </div>
          {timestamp && (
            <span className={`text-xs flex-shrink-0 ${isActive ? 'text-primary' : 'text-on-surface-variant'}`}>
              {timestamp}
            </span>
          )}
        </div>

        {/* External badge — Stitch: text-[10px] font-bold tracking-wider px-1.5 py-0.5 rounded border border-secondary */}
        {isExternal && (
          <div className="flex items-center gap-2 mb-1">
            <span className="text-[10px] font-bold tracking-wider px-1.5 py-0.5 rounded border border-secondary text-secondary">
              EXTERNAL
            </span>
          </div>
        )}

        {/* Row 2: preview — Stitch: font-body-md text-body-sm text-on-surface-variant truncate */}
        <p className={`text-sm truncate m-0 ${isActive ? 'font-medium' : 'text-on-surface-variant'}`}>
          {preview || '\u00A0'}
        </p>
      </div>

      {/* Unread dot — Stitch: absolute right-3 top-1/2 w-2 h-2 bg-primary rounded-full */}
      {hasUnread && (
        <div className="absolute right-3 top-1/2 -translate-y-1/2 w-2 h-2 bg-primary rounded-full" />
      )}
    </button>
  )
}
