import type { ReactNode } from 'react'
import { formatCount } from '../../lib/format'
import { Pressable } from '../primitives'
import { ThreadGlyph, UnreadGlyph } from './PhoneGlyphs'

/** The title row of a tab (mockup `.ttl`): the name, and the buttons that act on the list beside it. */
export function PhoneListTitle({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex items-center gap-2 pt-3.5 pr-3 pb-1.5 pl-4">
      <h1 className="flex-1 min-w-0 m-0 truncate font-display text-[1.625rem] leading-[1.2] font-bold tracking-tight text-ink">
        {title}
      </h1>
      {children}
    </div>
  )
}

/**
 * The pill that shows only what is unread (design/mockups/chat-mobile-home.html
 * `.tbtn.icon`): 52 wide, 40 high on a list title, 36 on a section header.
 * `aria-pressed`; the device remembers it where the list says so.
 */
export function UnreadToggle({ pressed, onChange, label, compact = false }: {
  pressed: boolean
  onChange: (on: boolean) => void
  /** Names what it filters: "Chỉ hiện chưa đọc", or "… trong <mục>". */
  label: string
  compact?: boolean
}) {
  return (
    <Pressable
      aria-pressed={pressed}
      aria-label={label}
      onClick={() => onChange(!pressed)}
      className={`inline-flex items-center justify-center w-13 rounded-full shrink-0 transition-colors duration-quick
        motion-reduce:transition-none ${compact ? 'h-9' : 'h-10'}
        ${pressed ? 'bg-accent-wash text-accent ring-1 ring-inset ring-accent' : 'bg-sunk text-ink hover:bg-hover'}`}
    >
      <UnreadGlyph size={compact ? 16 : 20} />
    </Pressable>
  )
}

/** "Thread" beside the title of Trang chủ, with how many followed threads have news. */
export function ThreadButton({ news, onClick }: { news: number; onClick: () => void }) {
  return (
    <Pressable
      onClick={onClick}
      aria-haspopup="dialog"
      aria-label={news > 0 ? `Thread, ${news} thread có tin mới` : 'Thread'}
      className="relative inline-flex items-center gap-2 h-10 pl-3 pr-4 rounded-full bg-sunk text-ink text-sm font-semibold
        shrink-0 hover:bg-hover"
    >
      <ThreadGlyph size={16} />
      Thread
      {news > 0 && (
        <span
          aria-hidden="true"
          className="absolute -top-1 -right-0.5 inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full
            bg-accent text-on-accent text-2xs font-semibold tnum ring-2 ring-base"
        >
          {formatCount(news)}
        </span>
      )}
    </Pressable>
  )
}
