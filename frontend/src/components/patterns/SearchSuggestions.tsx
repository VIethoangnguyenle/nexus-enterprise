import { useCallback, useLayoutEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { Clock, Search, X } from 'lucide-react'
import { useMotionPresets } from '../../lib/motion'
import { IconButton, Pressable } from '../primitives'
import type { ShownGroup, ShownItem, SuggestionPanel } from '../../hooks/useSuggestionPanel'

const GAP = 6
const MIN_WIDTH = 380
const MAX_HEIGHT = 480
/** Space kept between the popover and the window's edges. */
const EDGE = 12

/**
 * Where the popover sits: under the control around the box, at least as wide as it (and 380), slid left rather than
 * cut by the window's right edge, and no taller than the room below the box.
 */
function usePlacement(panel: SuggestionPanel): CSSProperties {
  const { anchorRef, visible, variant } = panel
  const [pos, setPos] = useState<CSSProperties>({})
  const place = useCallback(() => {
    const box = anchorRef.current
    const a = (box?.closest('label') ?? box)?.getBoundingClientRect()
    if (!a) return
    const width = Math.min(Math.max(a.width, MIN_WIDTH), window.innerWidth - 2 * EDGE)
    const left = Math.max(EDGE, Math.min(a.left, window.innerWidth - EDGE - width))
    const top = a.bottom + GAP
    setPos({ top, left, width, maxHeight: Math.min(MAX_HEIGHT, window.innerHeight - top - EDGE) })
  }, [anchorRef])

  useLayoutEffect(() => {
    if (!visible || variant !== 'popover') return
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [visible, variant, place])
  return pos
}

interface Props {
  panel: SuggestionPanel
  /** Phone screen only: the one line shown when there is nothing to suggest. */
  emptyText?: string
  label?: string
}

/** Presses inside never take focus from the box; a row acts on click. */
const keepFocus = (e: { preventDefault: () => void }) => e.preventDefault()

/**
 * What an empty search box suggests (DESIGN.md "Gợi ý trong ô tìm"). On a desktop, a listbox in a popover under
 * the box, driven from the box by keyboard. On a phone, groups of buttons that cover the list under the box, each
 * recent search with its own ✕ so it can be removed by touch and by a screen reader alike.
 */
export function SearchSuggestions({ panel, emptyText, label = 'Gợi ý tìm kiếm' }: Props) {
  const m = useMotionPresets()
  const pos = usePlacement(panel)
  if (!panel.visible) return null

  if (panel.variant === 'screen') {
    return (
      <div className="min-h-0 overflow-y-auto" onMouseDown={keepFocus}>
        {panel.groups.length === 0 ? (
          <div className="grid place-items-center gap-2 px-8 py-12 text-center text-ink-muted">
            <Search size={24} strokeWidth={1.75} aria-hidden="true" />
            <p className="m-0 text-sm">{emptyText}</p>
          </div>
        ) : (
          <div role="region" aria-label={label} className="grid pb-6">
            {panel.groups.map((g) => (
              <Group key={g.uid} group={g} panel={panel} phone>
                {g.items.map((item) => <PhoneRow key={item.uid} item={item} recent={!!g.recent} panel={panel} />)}
              </Group>
            ))}
          </div>
        )}
      </div>
    )
  }

  let index = 0
  const removable = panel.groups.some((g) => g.items.some((it) => it.onRemove))
  return createPortal(
    <motion.div
      {...m.popover}
      onMouseDown={keepFocus}
      style={pos}
      className="fixed z-dropdown overflow-y-auto rounded-overlay bg-overlay shadow-overlay"
    >
      <div id={panel.id} role="listbox" aria-label={label} className="grid py-1.5">
        {panel.groups.map((g) => (
          <Group key={g.uid} group={g} panel={panel}>
            {g.items.map((item) => (
              <Option key={item.uid} item={item} index={index++} recent={!!g.recent} panel={panel} />
            ))}
          </Group>
        ))}
      </div>
      <div aria-hidden="true" className="flex flex-wrap gap-x-3.5 mt-1 px-3.5 pt-2 pb-2.5 border-t border-line text-xs text-ink-subtle">
        <span><Kbd>↑</Kbd> <Kbd>↓</Kbd> chọn</span>
        <span><Kbd>Enter</Kbd> mở</span>
        {removable && <span><Kbd>Delete</Kbd> xoá</span>}
        <span><Kbd>Esc</Kbd> đóng</span>
      </div>
    </motion.div>,
    document.body,
  )
}

function Group({ group, panel, phone = false, children }: {
  group: ShownGroup; panel: SuggestionPanel; phone?: boolean; children: ReactNode
}) {
  const labelId = `${panel.id}-g${group.uid}`
  return (
    <div role="group" aria-labelledby={labelId} aria-busy={group.loading || undefined} className="grid">
      <div id={labelId} className={`text-ink-muted font-semibold ${phone ? 'px-4 pt-4 pb-1.5 text-sm' : 'px-3.5 pt-3 pb-1 text-xs'}`}>
        {group.label}
      </div>
      {group.items.length === 0 && group.loading ? [0, 1].map((n) => <SkeletonRow key={n} phone={phone} />) : children}
    </div>
  )
}

function Lead({ item, recent, phone }: { item: ShownItem; recent: boolean; phone: boolean }) {
  if (item.icon) return <span className="grid place-items-center">{item.icon}</span>
  if (!recent) return <span />
  return (
    <span className={`grid place-items-center rounded-md bg-sunk text-ink-muted ${phone ? 'size-9' : 'size-8'}`}>
      <Clock size={16} strokeWidth={1.75} aria-hidden="true" />
    </span>
  )
}

function RowText({ item, phone }: { item: ShownItem; phone: boolean }) {
  return (
    <span className="min-w-0">
      <span className={`block truncate font-medium ${phone ? 'text-[0.9375rem]' : ''}`}>{item.title}</span>
      {item.meta && <span className={`block truncate text-ink-muted ${phone ? 'text-[0.8125rem]' : 'text-xs'}`}>{item.meta}</span>}
    </span>
  )
}

/** A desktop row. Its ✕ is for the pointer only: from the keyboard, Delete removes the marked search. */
function Option({ item, index, recent, panel }: { item: ShownItem; index: number; recent: boolean; panel: SuggestionPanel }) {
  const selected = index === panel.active
  return (
    <div
      id={panel.optionId(index)}
      role="option"
      aria-selected={selected}
      onClick={() => panel.select(item)}
      onMouseEnter={() => panel.mark(item)}
      className={`group grid grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-3 min-h-11 mx-1.5 pl-2 pr-1.5 py-1
        rounded-md cursor-pointer text-sm text-ink transition-colors duration-quick
        ${selected ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <Lead item={item} recent={recent} phone={false} />
      <RowText item={item} phone={false} />
      {item.onRemove ? (
        <IconButton
          type="button"
          tabIndex={-1}
          aria-hidden="true"
          aria-label={`Xoá “${item.title}” khỏi Tìm gần đây`}
          title="Xoá khỏi Tìm gần đây"
          shape="round"
          onClick={(e) => {
            e.stopPropagation()
            item.onRemove!()
          }}
          className={selected ? '' : 'opacity-0 group-hover:opacity-100'}
        >
          <X size={16} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
      ) : (
        <span className="pr-2 text-xs text-ink-subtle tabular-nums">{item.trailing}</span>
      )}
    </div>
  )
}

/** A phone row: a button that chooses, and for a recent search a ✕ beside it with its own name. */
function PhoneRow({ item, recent, panel }: { item: ShownItem; recent: boolean; panel: SuggestionPanel }) {
  return (
    <div className="flex items-center pr-1">
      <Pressable
        aria-label={recent ? `Tìm lại ${item.title}` : undefined}
        onClick={() => panel.select(item)}
        className="flex-1 min-w-0 grid grid-cols-[36px_minmax(0,1fr)_auto] items-center gap-3.5 min-h-13 pl-4 pr-2 py-1.5
          text-ink active:bg-hover"
      >
        <Lead item={item} recent={recent} phone />
        <RowText item={item} phone />
        <span className="text-xs text-ink-subtle tabular-nums">{item.trailing}</span>
      </Pressable>
      {item.onRemove && (
        <IconButton
          type="button"
          aria-label={`Xoá “${item.title}” khỏi Tìm gần đây`}
          shape="round"
          size="lg"
          className="w-11 h-11"
          onClick={item.onRemove}
        >
          <X size={16} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
      )}
    </div>
  )
}

function SkeletonRow({ phone }: { phone: boolean }) {
  return (
    <div aria-hidden="true" className={`grid grid-cols-[auto_1fr] items-center gap-3 ${phone ? 'min-h-13 px-4' : 'min-h-11 mx-1.5 px-2'}`}>
      <span className={`skeleton rounded-md ${phone ? 'size-9' : 'size-8'}`} />
      <span className="skeleton h-3.5 w-2/3 rounded-sm" />
    </div>
  )
}

function Kbd({ children }: { children: string }) {
  return <span className="px-1.5 py-0.5 rounded-sm bg-sunk font-mono text-[0.6875rem] font-semibold text-ink-muted">{children}</span>
}
