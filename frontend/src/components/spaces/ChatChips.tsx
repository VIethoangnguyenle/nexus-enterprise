import { useEffect, useRef } from 'react'
import { formatCount } from '../../lib/format'
import { Pressable } from '../primitives'
import { ScrollFade } from '../primitives/ScrollFade'
import type { ChatTab } from '../../lib/nav-destinations'

const CHIPS: { id: ChatTab; label: string }[] = [
  { id: 'all', label: 'Tất cả' },
  { id: 'sections', label: 'Mục' },
  { id: 'unread', label: 'Chưa đọc' },
  { id: 'mentions', label: 'Được nhắc' },
  { id: 'direct', label: 'Trực tiếp' },
  { id: 'spaces', label: 'Nhóm' },
]

/**
 * The filter chips under the title of Chat on a phone (DESIGN.md "Chat trên điện thoại", mockup
 * mobile-shell.html §1): Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm, one row that scrolls
 * sideways, 36px high. The chosen chip is always scrolled into view. Chưa đọc and Được nhắc carry how many
 * conversations / mentions are waiting; the figure is read out in the chip's name.
 */
export function ChatChips({ current, onSelect, counts }: {
  current: ChatTab
  onSelect: (chip: ChatTab) => void
  counts: Partial<Record<ChatTab, number>>
}) {
  const selected = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    selected.current?.scrollIntoView?.({ block: 'nearest', inline: 'center' })
  }, [current])

  return (
    <ScrollFade role="group" aria-label="Lọc trò chuyện" className="flex gap-2 px-4 pb-2">
      {CHIPS.map(({ id, label }) => {
        const on = current === id
        const n = counts[id] ?? 0
        return (
          <Pressable
            key={id}
            ref={on ? selected : undefined}
            aria-pressed={on}
            aria-label={n > 0 ? `${label}, ${n} chưa đọc` : label}
            onClick={() => onSelect(id)}
            className={`press inline-flex items-center gap-1.5 shrink-0 h-9 px-3.5 rounded-full text-sm
              ${on ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover'}`}
          >
            {label}
            {n > 0 && (
              <span aria-hidden="true" className="tnum text-xs font-semibold text-accent">{formatCount(n)}</span>
            )}
          </Pressable>
        )
      })}
    </ScrollFade>
  )
}
