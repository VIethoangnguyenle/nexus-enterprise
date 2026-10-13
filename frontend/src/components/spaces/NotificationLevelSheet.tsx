import { useEffect, useRef, useState } from 'react'
import type { NotificationLevel } from '../../api/notification-levels'
import type { Conversation } from '../../lib/conversations'
import { levelOptions, optionFor } from '../../lib/notification-level-copy'
import { LEVEL_ICON } from './ConversationTitleMenu'
import { Pressable } from '../primitives'
import { NavSheet } from './NavSheet'

/** The radio shows its new state this long before the sheet goes (design: 160ms). */
const CLOSE_DELAY_MS = 160

/**
 * The phone's notification levels (design/mockups/notification-levels.html §5):
 * a bottom sheet titled "Thông báo" with the conversation's name, a radio group
 * of 64px rows. A touch changes the radio at once, saves like the desktop
 * menu, and the sheet closes 160ms later.
 */
export function NotificationLevelSheet({ conversation: c, level, onChoose, onClose }: {
  conversation: Pick<Conversation, 'kind' | 'title'>
  level: NotificationLevel
  onChoose: (level: NotificationLevel) => void
  onClose: () => void
}) {
  const [picked, setPicked] = useState<NotificationLevel>(optionFor(c.kind, level).level)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current) }, [])

  const choose = (next: NotificationLevel) => {
    setPicked(next)
    if (next !== optionFor(c.kind, level).level) onChoose(next)
    timer.current = setTimeout(onClose, CLOSE_DELAY_MS)
  }

  return (
    <NavSheet label="Thông báo" onClose={onClose}>
      <div className="px-3.5 pt-1 pb-2">
        <h2 className="m-0 text-lg font-semibold text-ink">Thông báo</h2>
        <p className="m-0 text-small text-ink-muted truncate">{c.title}</p>
      </div>
      <div role="radiogroup" aria-label="Thông báo" className="grid gap-1">
        {levelOptions(c.kind).map((o) => {
          const Icon = LEVEL_ICON[o.level]
          const checked = o.level === picked
          return (
            <Pressable
              key={o.level}
              role="radio"
              aria-checked={checked}
              onClick={() => choose(o.level)}
              className="flex items-center gap-3.5 w-full min-h-16 px-3.5 rounded-lg text-left cursor-pointer border-none
                bg-transparent hover:bg-hover active:bg-hover transition-colors duration-quick focus-ring"
            >
              <Icon size={20} strokeWidth={1.75} aria-hidden="true" className={`shrink-0 ${checked ? 'text-accent' : 'text-ink-muted'}`} />
              <span className="grid gap-0.5 min-w-0 flex-1">
                <span className="flex items-center gap-2">
                  <b className="text-16 font-semibold text-ink">{o.title}</b>
                  {o.isDefault && <span className="px-1.5 rounded-sm bg-sunk text-2xs font-semibold text-ink-muted">Mặc định</span>}
                </span>
                <span className="text-small text-ink-muted">{o.description}</span>
              </span>
              <span
                aria-hidden="true"
                className={`grid place-items-center w-5.5 h-5.5 rounded-full border-2 shrink-0
                  ${checked ? 'border-accent' : 'border-line'}`}
              >
                {checked && <span className="w-2.5 h-2.5 rounded-full bg-accent" />}
              </span>
            </Pressable>
          )
        })}
      </div>
    </NavSheet>
  )
}
