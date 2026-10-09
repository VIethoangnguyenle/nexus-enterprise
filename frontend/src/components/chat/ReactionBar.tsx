import { AnimatePresence, motion } from 'motion/react'
import { SmilePlus } from 'lucide-react'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'

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

/**
 * Reaction chips under a message. Yours are tinted with the accent wash. A
 * chip that appears pops in (scale 0.6 → 1, spring, 280ms; the one place the
 * spring curve is allowed); chips present on load do not animate.
 */
export function ReactionBar({ reactions, currentUserId, onToggle, onAddReaction }: ReactionBarProps) {
  const m = useMotionPresets()
  if (!reactions?.length) return null

  const pop = m.reduced
    ? { initial: { opacity: 0 }, animate: { opacity: 1, transition: { duration: DURATION.reducedFade } } }
    : {
        initial: { opacity: 0, scale: 0.6 },
        animate: { opacity: 1, scale: 1, transition: { duration: DURATION.layout, ease: EASE.spring } },
      }

  return (
    <div className="flex flex-wrap items-center gap-1.5 mt-1.5">
      {/* eslint-disable no-restricted-syntax -- Chip cảm xúc là nút bật/tắt dạng viên thuốc
          (emoji + bộ đếm, aria-pressed). Button không có dạng viên thuốc cao 26px; FilterChip là
          bộ lọc bo 8px. Cùng một họ với nút "+" cuối hàng nên giữ chung một khai báo. */}
      <AnimatePresence initial={false}>
        {reactions.map((r) => {
          const mine = r.user_ids?.includes(currentUserId)
          return (
            <motion.button
              key={r.emoji}
              type="button"
              {...pop}
              exit={{ opacity: 0, transition: { duration: 0.12 } }}
              aria-pressed={!!mine}
              aria-label={`${r.emoji} ${r.count}${mine ? ', có bạn' : ''}`}
              onClick={() => onToggle(r.emoji)}
              className={`press inline-flex items-center gap-1.5 h-6.5 px-2.25 rounded-full border-none
                cursor-pointer text-small text-ink focus-ring
                ${mine ? 'bg-accent-wash' : 'bg-sunk hover:bg-hover'}`}
            >
              <span>{r.emoji}</span>
              <span className="font-semibold tnum">{r.count}</span>
            </motion.button>
          )
        })}
      </AnimatePresence>
      <button
        type="button"
        onClick={onAddReaction}
        aria-label="Thêm cảm xúc"
        title="Thêm cảm xúc"
        className="press inline-grid place-items-center h-6.5 w-8 rounded-full border-none bg-transparent
          cursor-pointer text-ink-muted hover:bg-hover hover:text-ink focus-ring
          opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
      >
        <SmilePlus size={15} strokeWidth={1.75} />
      </button>
      {/* eslint-enable no-restricted-syntax */}
    </div>
  )
}
