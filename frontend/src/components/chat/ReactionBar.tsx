import { Plus } from 'lucide-react'

interface ReactionGroup {
  emoji: string
  count: number
  user_ids: string[]
}

interface ReactionBarProps {
  reactions: ReactionGroup[]
  currentUserId: string
  onToggle: (emoji: string) => void
  onAddReaction: () => void
}

/** Reaction chips matching Stitch nexus-chat.html:
 *  px-2 py-1 rounded-full bg-surface-container-high text-on-surface-variant
 *  border border-outline-variant/30, hover:bg-surface-container-highest.
 *  Active (user reacted): border-primary/40 bg-primary/10 text-primary.
 *  Count: font-label-caps text-label-caps. */
export function ReactionBar({ reactions, currentUserId, onToggle, onAddReaction }: ReactionBarProps) {
  if (!reactions?.length) return null

  return (
    <div className="flex flex-wrap items-center gap-1 mt-1">
      {reactions.map((r) => {
        const isActive = r.user_ids?.includes(currentUserId)
        return (
          <button
            key={r.emoji}
            onClick={() => onToggle(r.emoji)}
            className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-sm
              cursor-pointer transition-colors border-none bg-transparent
              ${isActive
                ? 'bg-primary/10 text-primary border border-primary/30'
                : 'bg-surface-container-high text-on-surface-variant border border-outline-variant/30 hover:bg-surface-container-highest'
              }`}
          >
            <span className="text-sm">{r.emoji}</span>
            <span className="font-label-caps text-label-caps">{r.count}</span>
          </button>
        )
      })}
      <button
        onClick={onAddReaction}
        className="w-7 h-7 flex items-center justify-center rounded-full
          text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high
          border border-outline-variant/30 bg-transparent cursor-pointer transition-colors"
        title="Add reaction"
      >
        <Plus size={14} />
      </button>
    </div>
  )
}
