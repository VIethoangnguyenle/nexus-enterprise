import { memo, useRef, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import type { AssetRequest } from '../../api/assets'
import { assetPerson, requestPill, urgencyPill } from '../../lib/asset-model'
import { formatDateTime, formatListTime } from '../../lib/format'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import type { PeopleDirectory } from '../../lib/people'
import { PersonChip, Pressable } from '../primitives'
import { AssetPill } from './AssetPill'

/**
 * Yêu cầu · người yêu cầu · mức độ · gửi lúc (+ trạng thái, when the list mixes
 * statuses) — mockup §3. The status column is dropped on the "Đang chờ" filter,
 * where every row would say the same.
 */
const COLS = {
  plain: `grid grid-cols-[minmax(0,1fr)_auto] @2xl:grid-cols-[minmax(0,1fr)_176px_112px_96px]
    gap-x-3 items-center px-2.5`,
  withStatus: `grid grid-cols-[minmax(0,1fr)_auto] @2xl:grid-cols-[minmax(0,1fr)_168px_104px_88px_136px]
    gap-x-3 items-center px-2.5`,
}

interface RequestTableProps {
  label: string
  requests: AssetRequest[]
  loading: boolean
  people: PeopleDirectory
  /** Show the status column (any filter but "Đang chờ"). */
  showStatus: boolean
  openId: string | null
  onOpen: (request: AssetRequest) => void
  /** Changes with the filter; rows enter in sequence when the list is new. */
  scope: string
}

export function RequestTable({ label, requests, loading, people, showStatus, openId, onOpen, scope }: RequestTableProps) {
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)
  const cols = showStatus ? COLS.withStatus : COLS.plain

  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && requests.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(requests.map((r, n) => [r.id, n])) }
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

  return (
    <div role="table" aria-label={label} aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${cols} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Yêu cầu</span>
        <span role="columnheader" className="hidden @2xl:block">Người yêu cầu</span>
        <span role="columnheader" className="hidden @2xl:block">Mức độ</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Gửi lúc</span>
        {showStatus && <span role="columnheader" className="hidden @2xl:block">Trạng thái</span>}
        <span role="columnheader" className="@2xl:hidden text-right">Mức độ</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2].map((i) => (
            <div key={i} role="row" className={`${cols} h-14 border-b border-line`}>
              <span className="grid gap-1.5">
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${60 - i * 10}%` }} />
                <span className="skeleton h-3 w-1/2 rounded-sm" />
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-5.5 w-16 rounded-full hidden @2xl:block" />
              <span className="skeleton h-3.5 w-2/3 rounded-sm ml-auto hidden @2xl:block" />
              {showStatus && <span className="skeleton h-5.5 w-20 rounded-full hidden @2xl:block" />}
              <span className="skeleton h-5.5 w-16 rounded-full @2xl:hidden" />
            </div>
          ))
        ) : (
          <AnimatePresence key={scope}>
            {requests.map((request) => {
              const index = firstBatch.current?.ids.get(request.id)
              const first = index !== undefined
              return (
                <motion.div
                  key={request.id}
                  {...withDelay(first ? m.route : m.row, first ? staggerDelay(index, m.reduced) : 0)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <RequestRow request={request} cols={cols} people={people} showStatus={showStatus} open={openId === request.id} onOpen={onOpen} />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
    </div>
  )
}

const RequestRow = memo(function RequestRow({ request, cols, people, showStatus, open, onOpen }: {
  request: AssetRequest
  cols: string
  people: PeopleDirectory
  showStatus: boolean
  open: boolean
  onOpen: (r: AssetRequest) => void
}) {
  const who = assetPerson(people, request.requester_id, request.requester_name)
  const sent = formatListTime(request.created_at)
  return (
    <div
      role="row"
      className={`${cols} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span role="cell" className="grid gap-0.5 min-w-0 py-2">
        <Pressable
          data-row
          aria-current={open || undefined}
          onClick={() => onOpen(request)}
          className="min-w-0 truncate font-medium text-ink text-left cursor-pointer outline-none
            after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
            focus-visible:after:-outline-offset-2"
        >
          {request.type_name || 'Tài sản'}
        </Pressable>
        <small className="text-xs text-ink-muted truncate">
          <span className="@2xl:hidden">{who.name} · {sent} · </span>
          {request.justification}
        </small>
      </span>
      <span role="cell" className="hidden @2xl:flex min-w-0">
        <PersonChip name={who.name} hueKey={who.hueKey} avatarUrl={who.avatarUrl} />
      </span>
      <span role="cell" className="hidden @2xl:flex">
        <AssetPill pill={urgencyPill(request.urgency)} />
      </span>
      <span role="cell" className="hidden @2xl:block text-right text-ink-muted tnum" title={formatDateTime(request.created_at)}>
        {sent}
      </span>
      {showStatus && (
        <span role="cell" className="hidden @2xl:flex">
          <AssetPill pill={requestPill(request.status)} />
        </span>
      )}
      <span role="cell" className="@2xl:hidden flex justify-end">
        <AssetPill pill={showStatus ? requestPill(request.status) : urgencyPill(request.urgency)} />
      </span>
    </div>
  )
})
