import { type ReactNode } from 'react'
import { AnimatePresence, motion, useIsPresent, type TargetAndTransition } from 'motion/react'
import { Clock, Search, X } from 'lucide-react'
import { DURATION, EASE, useMotionPresets, type MotionPresets } from '../../lib/motion'
import { IconButton, Pressable } from '../primitives'
import type { ShownGroup, ShownItem, SuggestionPanel } from '../../hooks/useSuggestionPanel'
import { Marked } from './Marked'

/** Rows past this one enter together with it, so a long list never keeps its tail waiting. */
const STAGGER_ROWS = 7
/** The panel's rows start this long after the surface does, and each is this much later than the one above. */
const ROW_DELAY = 0.06
const ROW_STEP = 0.02

interface Props {
  panel: SuggestionPanel
  /** Shown when there is nothing to suggest: on a phone screen always, on a desktop surface when it was asked to open empty. */
  emptyText?: string
  /** A second, quieter line under `emptyText` on a phone. */
  emptyHint?: string
  label?: string
  /** Keep drawing the rows although the panel has closed: the surface is still folding back into the box. */
  keep?: boolean
  /**
   * A link beside a group's label that widens the search to that group ("Xem tất cả · 24" of the one search of
   * every module, DESIGN.md §6 "Tìm cho mọi tác vụ"); none when it answers nothing.
   */
  more?: (group: ShownGroup) => GroupMore | undefined
}

/** What a group's "Xem tất cả" link says and does. */
export interface GroupMore {
  label: string
  onSelect: () => void
}

/** Presses inside never take focus from the box; a row acts on click. */
const keepFocus = (e: { preventDefault: () => void }) => e.preventDefault()

/**
 * How a row comes and goes (DESIGN.md "Gợi ý trong ô tìm"): on opening it slides down 4px and fades in, 160ms
 * after a short wait and 20ms later than the row above; while words are typed it only fades (120ms); removing it
 * fades it out and folds its height (160ms) so the rows below glide up. Under reduced motion only fades remain.
 */
function rowMotion(m: MotionPresets, index: number, typing: boolean, phone: boolean) {
  const fade = DURATION.reducedFade
  const exit: TargetAndTransition = m.reduced
    ? { opacity: 0, transition: { duration: fade } }
    : { opacity: 0, height: 0, overflow: 'hidden', transition: { duration: DURATION.quick, ease: EASE.in } }
  if (m.reduced || typing) {
    return { initial: { opacity: 0 }, animate: { opacity: 1, height: 'auto', transition: { duration: fade, ease: EASE.out } }, exit }
  }
  return {
    // A phone's whole panel slides; its rows only fade, one after another.
    initial: phone ? { opacity: 0 } : { opacity: 0, y: -4 },
    // `height: auto` brings a row back to its full height when it returns while folding away (it would otherwise stay as folded as it got).
    animate: {
      opacity: 1,
      y: 0,
      height: 'auto',
      transition: { duration: DURATION.quick, ease: EASE.out, delay: ROW_DELAY + Math.min(index, STAGGER_ROWS) * ROW_STEP },
    },
    exit,
  }
}

/**
 * What an empty search box suggests, and on Chat what the typed words suggest (DESIGN.md "Gợi ý trong ô tìm"). On a
 * desktop, the rows of a listbox inside the box's own surface (`SearchBox`), driven from the box by keyboard. On a
 * phone, groups of buttons that cover the list under the box. A recent search has its own ✕ and, from two on, the
 * group a "Xoá tất cả".
 */
export function SearchSuggestions({ panel, emptyText, emptyHint, label = 'Gợi ý tìm kiếm', keep = false, more }: Props) {
  const m = useMotionPresets()
  const phone = panel.variant === 'screen'
  if (!panel.visible && !keep) return null

  const rowsCount = panel.groups.reduce((n, g) => n + g.items.length, 0)
  const loading = panel.groups.some((g) => g.loading)
  const live = <p aria-live="polite" className="sr-only">{panel.said || (loading ? 'Đang tìm…' : `${rowsCount} gợi ý`)}</p>

  if (phone) {
    return (
      <motion.div
        // The panel rises 8px as it fades in, a beat after the screen under it has started to fade.
        initial={m.reduced ? { opacity: 0 } : { opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0, transition: { duration: m.reduced ? DURATION.reducedFade : DURATION.base, ease: EASE.out, delay: m.reduced ? 0 : ROW_DELAY } }}
        className="min-h-0 overflow-y-auto"
        onMouseDown={keepFocus}
      >
        {live}
        {panel.groups.length === 0 ? (
          <div className="grid justify-items-center gap-2 px-8 py-12 text-center text-ink-muted">
            <Search size={24} strokeWidth={1.75} aria-hidden="true" />
            <p className="m-0 text-sm">{emptyText}</p>
            {emptyHint && <p className="m-0 text-small">{emptyHint}</p>}
          </div>
        ) : (
          <div role="region" aria-label={label} className="grid pb-6">
            <Groups panel={panel} m={m} phone more={more} />
            {panel.note && <p className="m-0 px-4 py-3 text-small text-ink-muted">{panel.note}</p>}
          </div>
        )}
      </motion.div>
    )
  }

  return (
    <>
      {live}
      {panel.groups.length === 0 ? (
        <div className="grid justify-items-center gap-2 px-6 py-8 text-center text-ink-muted">
          <Search size={24} strokeWidth={1.75} aria-hidden="true" />
          <p className="m-0 text-sm">{emptyText}</p>
        </div>
      ) : (
        <div id={panel.id} role="listbox" aria-label={label} className="grid py-1.5">
          <Groups panel={panel} m={m} more={more} />
          {panel.note && <p className="m-0 px-3.5 pt-1.5 pb-1 text-small text-ink-muted">{panel.note}</p>}
        </div>
      )}
    </>
  )
}

/** The groups and their rows. Rows that leave are drawn until they have folded away. */
function Groups({ panel, m, phone = false, more }: {
  panel: SuggestionPanel; m: MotionPresets; phone?: boolean; more?: Props['more']
}) {
  let index = 0
  return (
    <AnimatePresence initial>
      {panel.groups.map((g) => (
        <GroupShell key={g.uid} m={m} labelId={`${panel.id}-${g.domId}`} busy={g.loading}>
          <GroupLabel group={g} panel={panel} phone={phone} more={more?.(g)} />
          {g.items.length === 0 && g.loading ? (
            [0, 1].map((n) => <SkeletonRow key={n} phone={phone} />)
          ) : (
            <AnimatePresence initial>
              {g.items.map((item) => {
                const at = index++
                return (
                  <RowShell key={item.uid} m={m} index={at} typing={panel.hasText} phone={phone}>
                    {phone
                      ? <PhoneRow item={item} recent={!!g.recent} panel={panel} />
                      : <Option item={item} index={at} recent={!!g.recent} panel={panel} />}
                  </RowShell>
                )
              })}
            </AnimatePresence>
          )}
        </GroupShell>
      ))}
    </AnimatePresence>
  )
}

/** A group folds away as a whole (label and rows) when its last row is removed. */
function GroupShell({ m, labelId, busy, children }: { m: MotionPresets; labelId: string; busy?: boolean; children: ReactNode }) {
  const present = useIsPresent()
  return (
    <motion.div
      role="group"
      aria-labelledby={labelId}
      aria-busy={busy || undefined}
      aria-hidden={!present || undefined}
      // A group that comes back while folding away returns to its full height, not the one it had got to.
      animate={{ opacity: 1, height: 'auto' }}
      exit={m.reduced
        ? { opacity: 0, transition: { duration: DURATION.reducedFade } }
        : { opacity: 0, height: 0, overflow: 'hidden', transition: { duration: DURATION.quick, ease: EASE.in } }}
      className="grid"
    >
      {children}
    </motion.div>
  )
}

/** One row's entrance and exit. While it is leaving it is hidden from assistive technology and from clicks. */
function RowShell({ m, index, typing, phone, children }: {
  m: MotionPresets; index: number; typing: boolean; phone: boolean; children: ReactNode
}) {
  const present = useIsPresent()
  return (
    <motion.div
      role="presentation"
      aria-hidden={!present || undefined}
      {...rowMotion(m, index, typing, phone)}
      style={present ? undefined : { pointerEvents: 'none' }}
    >
      {children}
    </motion.div>
  )
}

function GroupLabel({ group, panel, phone, more }: { group: ShownGroup; panel: SuggestionPanel; phone: boolean; more?: GroupMore }) {
  const labelId = `${panel.id}-${group.domId}`
  const clearable = !!group.onClearAll && group.items.length >= 2
  if (group.hideLabel && !clearable && !more) return <span id={labelId} className="sr-only">{group.label}</span>
  return (
    <div className={`flex items-center justify-between gap-2 ${phone ? 'min-h-11 pl-4 pr-1 pt-4' : 'min-h-8 pl-3.5 pr-2 pt-2 pb-0.5'}`}>
      <div id={labelId} className={`text-ink-muted font-semibold ${phone ? 'text-sm' : 'text-xs'} ${group.hideLabel ? 'sr-only' : ''}`}>
        {group.label}
      </div>
      {clearable && (
        <Pressable
          onClick={() => panel.clearAll(group)}
          className={`shrink-0 rounded-md font-semibold text-accent hover:bg-accent-wash transition-colors duration-quick
            ${phone ? 'h-11 px-3 text-[0.8125rem]' : 'h-7 px-2 text-xs'}`}
        >
          Xoá tất cả
        </Pressable>
      )}
      {more && (
        <Pressable
          onClick={more.onSelect}
          aria-label={`${more.label} ${group.label}`}
          className={`shrink-0 rounded-md font-semibold text-accent hover:bg-accent-wash transition-colors duration-quick
            ${phone ? 'h-11 px-3 text-[0.8125rem]' : 'h-7 px-2 text-xs'}`}
        >
          {more.label}
        </Pressable>
      )}
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
      <span className={`block truncate ${item.strong ? 'font-semibold' : 'font-medium'} ${phone ? 'text-[0.9375rem]' : ''}`}>
        <Marked text={item.title} query={item.mark} />
      </span>
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
      aria-label={item.label}
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
            panel.remove(item)
          }}
          className={`transition-opacity duration-quick ease-out motion-reduce:transition-none ${selected ? '' : 'opacity-0 group-hover:opacity-100'}`}
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
        aria-label={item.label}
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
          onClick={() => panel.remove(item)}
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

/** A key cap, as the hints of the search surface and the "Xem tất cả" row draw them. */
export function Kbd({ children }: { children: string }) {
  return <span className="px-1.5 py-0.5 rounded-sm bg-sunk font-mono text-[0.6875rem] font-semibold text-ink-muted">{children}</span>
}
