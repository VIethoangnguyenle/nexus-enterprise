import { useEffect, useId, useRef, useState } from 'react'
import { Bell, BellOff, BellRing, Check, ChevronDown, Info, Star } from 'lucide-react'
import type { NotificationLevel } from '../../api/notification-levels'
import type { Conversation } from '../../lib/conversations'
import { levelOptions, optionFor } from '../../lib/notification-level-copy'
import { Heading, MenuItem, MenuSeparator, Popover, Pressable } from '../primitives'

export const LEVEL_ICON = { all: BellRing, main: Bell, off: BellOff } as const

/** The icon after the name of a conversation whose level is not the default, and the words a screen reader hears. */
function LevelMark({ level }: { level: NotificationLevel }) {
  if (level === 'main') return null
  const Icon = LEVEL_ICON[level]
  return (
    <>
      <Icon size={16} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted shrink-0" />
      <span className="sr-only">{level === 'off' ? 'đã tắt thông báo' : 'thông báo mọi tin'}</span>
    </>
  )
}

/**
 * The conversation's name in its header, as the button that opens its menu
 * (design/mockups/notification-levels.html §2, shortcuts.html §3): the
 * "Thông báo" levels as radios, then Gắn sao and the info panel. The icon of a
 * non-default level and the star of a starred conversation follow the name.
 * On a phone the name opens the info screen instead, where the level is set.
 */
export function ConversationTitleMenu({ conversation: c, level, starred, phone = false, onChooseLevel, onToggleStar, onOpenInfo }: {
  conversation: Pick<Conversation, 'kind' | 'title'>
  level: NotificationLevel
  starred: boolean
  phone?: boolean
  onChooseLevel: (level: NotificationLevel) => void
  onToggleStar: () => void
  onOpenInfo: () => void
}) {
  const [open, setOpen] = useState(false)
  const anchor = useRef<HTMLButtonElement>(null)
  const ids = useId()
  const current = optionFor(c.kind, level)
  const infoLabel = c.kind === 'space' ? 'Thông tin nhóm' : 'Thông tin'

  const body = (
    <>
      <Name title={c.title} level={level} starred={starred} />
      {!phone && (
        <ChevronDown
          size={16}
          strokeWidth={1.75}
          aria-hidden="true"
          className={`shrink-0 text-ink-muted transition-transform duration-quick motion-reduce:transition-none ${open ? 'rotate-180' : ''}`}
        />
      )}
    </>
  )

  return (
    <Heading as="h1" look="panel" className="min-w-0">
      <Pressable
        ref={anchor}
        {...(phone ? {} : { 'aria-haspopup': 'menu' as const, 'aria-expanded': open })}
        onClick={() => (phone ? onOpenInfo() : setOpen((v) => !v))}
        className="inline-flex items-center gap-1.5 min-h-9 max-w-full -ml-1.5 px-1.5 rounded-md border-none bg-transparent
          text-inherit font-inherit cursor-pointer focus-ring hover:bg-hover transition-colors duration-quick"
      >
        {body}
      </Pressable>
      {!phone && open && (
        <Popover
          open
          onClose={() => setOpen(false)}
          anchorRef={anchor}
          role="menu"
          label={`Tuỳ chọn cho ${c.title}`}
          className="w-86 max-w-[calc(100vw-1rem)]"
        >
          <FocusChecked />
          <div role="group" aria-labelledby={`${ids}-label`} className="grid gap-0.5">
            <div id={`${ids}-label`} className="px-2.5 pt-1 pb-1 text-xs font-semibold text-ink-muted">Thông báo</div>
            {levelOptions(c.kind).map((o) => {
              const Icon = LEVEL_ICON[o.level]
              const checked = o.level === current.level
              return (
                <MenuItem
                  key={o.level}
                  role="menuitemradio"
                  aria-checked={checked}
                  aria-labelledby={`${ids}-${o.level}-t`}
                  aria-describedby={`${ids}-${o.level}-d`}
                  className="h-auto! items-start py-2"
                  onClick={() => {
                    if (o.level !== current.level) onChooseLevel(o.level)
                  }}
                >
                  <Icon size={16} strokeWidth={1.75} aria-hidden="true" className={`mt-0.5 shrink-0 ${checked ? 'text-accent' : 'text-ink-muted'}`} />
                  <span className="grid gap-0.5 min-w-0 flex-1">
                    <span id={`${ids}-${o.level}-t`} className="flex items-center gap-2">
                      <b className="font-semibold">{o.title}</b>
                      {o.isDefault && (
                        <span className="px-1.5 rounded-sm bg-sunk text-2xs font-semibold text-ink-muted">Mặc định</span>
                      )}
                    </span>
                    <span id={`${ids}-${o.level}-d`} className="text-xs font-normal text-ink-muted">{o.description}</span>
                  </span>
                  {checked && <Check size={16} strokeWidth={1.75} aria-hidden="true" className="mt-0.5 text-accent shrink-0" />}
                </MenuItem>
              )
            })}
          </div>
          <MenuSeparator />
          <MenuItem icon={<Star size={16} strokeWidth={1.75} aria-hidden="true" />} onClick={onToggleStar}>
            {starred ? 'Bỏ gắn sao' : 'Gắn sao'}
          </MenuItem>
          <MenuItem icon={<Info size={16} strokeWidth={1.75} aria-hidden="true" />} onClick={onOpenInfo}>
            {infoLabel}
          </MenuItem>
        </Popover>
      )}
    </Heading>
  )
}

function Name({ title, level, starred }: { title: string; level: NotificationLevel; starred: boolean }) {
  return (
    <>
      <span className="truncate">{title}</span>
      <LevelMark level={level} />
      {starred && (
        <>
          <Star size={16} strokeWidth={1.75} fill="currentColor" aria-hidden="true" className="text-accent shrink-0" />
          <span className="sr-only">Đã gắn sao</span>
        </>
      )}
    </>
  )
}

/** The menu opens on the level in force, not on its first row (design: keyboard). Runs after the popover's own focus. */
function FocusChecked() {
  useEffect(() => {
    let second = 0
    const first = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => {
        document.querySelector<HTMLElement>('[role="menuitemradio"][aria-checked="true"]')?.focus()
      })
    })
    return () => {
      cancelAnimationFrame(first)
      cancelAnimationFrame(second)
    }
  }, [])
  return null
}
