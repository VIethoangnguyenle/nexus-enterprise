import { memo, useRef, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import type { ApprovalRow } from '../../hooks/useApproval'
import type { Arrival } from '../../hooks/useArrivals'
import { amountOfRequest, personFor, requestPill, requestTitle } from '../../lib/approval-model'
import { formatDateTime, formatListTime, formatMoney } from '../../lib/format'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import type { PeopleDirectory } from '../../lib/people'
import { personStyle } from '../../lib/person-hue'
import { Checkbox, PersonChip, Pressable } from '../primitives'
import { StatusPill } from './StatusPill'

/**
 * Đề nghị · người gửi · số tiền · trạng thái (mockup §3). The columns follow the
 * table's own width, not the window: with the detail panel open the table is
 * too narrow for four columns, so below `@2xl` the requester and amount move
 * under the title.
 */
const COLS = {
  plain: `grid grid-cols-[minmax(0,1fr)_auto] @2xl:grid-cols-[minmax(0,1fr)_176px_128px_136px]
    gap-x-3 items-center px-2.5`,
  selectable: `grid grid-cols-[32px_minmax(0,1fr)_auto] @2xl:grid-cols-[32px_minmax(0,1fr)_176px_128px_136px]
    gap-x-3 items-center px-2.5`,
}

export interface RealtimeTag {
  /** "Khải vừa duyệt" */
  text: string
  /** Colour key of the person who did it. Never rendered. */
  hueKey: string
}

interface ApprovalTableProps {
  /** Table name for assistive tech, e.g. "Đề nghị chờ tôi duyệt". */
  label: string
  rows: ApprovalRow[]
  loading: boolean
  people: PeopleDirectory
  /** Request open in the panel. */
  openId: string | null
  onOpen: (row: ApprovalRow) => void
  /** Ticking rows for batch approval; only the pending tab offers it. */
  selection?: { ids: Set<string>; onToggle: (id: string) => void; onToggleAll: () => void }
  /** Changes per tab; rows enter in sequence when the list is new. */
  scope: string
  /** Realtime arrivals, by version, from `useArrivals`. */
  fresh: Map<string, Arrival>
  versionOf: (row: ApprovalRow) => string
  tagOf: (row: ApprovalRow, arrival: Arrival) => RealtimeTag | undefined
  /** Every row has finished folding away (the last one left). */
  onRowsGone?: () => void
}

/**
 * The hairline table (DESIGN.md §6): header 12px/600, rows 56px divided by a
 * 1px line, no zebra, no outer border. A row's title is the real control — a
 * button stretched over the whole row; the tick box sits above it. ↑/↓ move
 * between rows.
 */
export function ApprovalTable(props: ApprovalTableProps) {
  const { rows, loading, scope, selection } = props
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)
  const cols = selection ? COLS.selectable : COLS.plain

  // Rows on screen when the list first shows enter in sequence; later arrivals enter alone.
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && rows.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(rows.map((r, n) => [r.request.id, n])) }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const items = Array.from(bodyRef.current?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
    const at = items.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    const next = items[e.key === 'ArrowDown' ? Math.min(at + 1, items.length - 1) : Math.max(at - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  const allTicked = rows.length > 0 && rows.every((r) => selection?.ids.has(r.request.id))
  const someTicked = !allTicked && rows.some((r) => selection?.ids.has(r.request.id))

  return (
    <div role="table" aria-label={props.label} aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${cols} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        {selection && (
          <span role="columnheader" className="grid place-items-center">
            <Checkbox
              label="Chọn tất cả đề nghị"
              checked={allTicked}
              indeterminate={someTicked}
              onChange={selection.onToggleAll}
              disabled={loading || rows.length === 0}
            />
          </span>
        )}
        <span role="columnheader">Đề nghị</span>
        <span role="columnheader" className="hidden @2xl:block">Người gửi</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Số tiền</span>
        <span role="columnheader" className="text-right">Trạng thái</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3].map((i) => (
            <div key={i} role="row" className={`${cols} h-14 border-b border-line`}>
              {selection && <span />}
              <span className="grid gap-1.5">
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${72 - i * 9}%` }} />
                <span className="skeleton h-3 w-1/4 rounded-sm" />
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-3.5 w-2/3 rounded-sm ml-auto hidden @2xl:block" />
              <span className="skeleton h-5.5 w-20 rounded-full ml-auto" />
            </div>
          ))
        ) : (
          // Keyed by tab: leaving one swaps the rows at once; only rows that
          // appear or go while the tab is open animate.
          <AnimatePresence key={scope} onExitComplete={props.onRowsGone}>
            {rows.map((row) => {
              const id = row.request.id
              const index = firstBatch.current?.ids.get(id)
              const first = index !== undefined
              const delay = first ? staggerDelay(index, m.reduced) : 0
              const arrival = props.fresh.get(props.versionOf(row))
              return (
                <motion.div
                  key={id}
                  {...withDelay(first ? m.route : m.row, delay)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <ApprovalRowView
                    row={row}
                    cols={cols}
                    people={props.people}
                    open={props.openId === id}
                    ticked={!!selection?.ids.has(id)}
                    selection={selection}
                    tag={arrival?.source === 'other' ? props.tagOf(row, arrival) : undefined}
                    onOpen={props.onOpen}
                  />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

interface RowViewProps {
  row: ApprovalRow
  cols: string
  people: PeopleDirectory
  open: boolean
  ticked: boolean
  selection: ApprovalTableProps['selection']
  tag?: RealtimeTag
  onOpen: (row: ApprovalRow) => void
}

const ApprovalRowView = memo(function ApprovalRowView({ row, cols, people, open, ticked, selection, tag, onOpen }: RowViewProps) {
  const { request } = row
  const requester = personFor(people, request.created_by, request.created_by_name)
  const amount = amountOfRequest(request)
  const title = requestTitle(request)
  const pill = requestPill(row)

  return (
    <div
      role="row"
      style={tag ? personStyle(tag.hueKey) : undefined}
      className={`${cols} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'} ${tag ? 'rt-wash' : ''}`}
    >
      {selection && (
        <span role="cell" className="relative z-dropdown grid place-items-center">
          <Checkbox label={`Chọn ${title}`} checked={ticked} onChange={() => selection.onToggle(request.id)} />
        </span>
      )}
      <span role="cell" className="grid gap-0.5 min-w-0 py-2">
        <span className="flex items-center gap-2 min-w-0">
          <Pressable
            data-row
            aria-current={open || undefined}
            onClick={() => onOpen(row)}
            className="min-w-0 line-clamp-2 break-words font-medium text-ink text-left cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {title}
          </Pressable>
          {tag && <span className="rt-tag shrink-0 text-xs font-semibold">{tag.text}</span>}
        </span>
        {/* Wide: when it was sent. Narrow: who and how much, which is what a phone needs to decide. */}
        <small className="text-xs text-ink-muted tnum truncate" title={formatDateTime(request.created_at)}>
          <span className="hidden @2xl:inline">Gửi {formatListTime(request.created_at)}</span>
          <span className="@2xl:hidden">
            {requester.name}{amount !== null ? ` · ${formatMoney(amount)}` : ''}
          </span>
        </small>
      </span>
      <span role="cell" className="hidden @2xl:flex min-w-0">
        <PersonChip name={requester.name} hueKey={requester.hueKey} avatarUrl={requester.avatarUrl} />
      </span>
      <span role="cell" className="hidden @2xl:block text-right tnum font-medium">
        {amount !== null ? formatMoney(amount) : ''}
      </span>
      <span role="cell" className="flex justify-end">
        <StatusPill pill={pill} />
      </span>
    </div>
  )
})
