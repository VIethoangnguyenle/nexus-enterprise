import { memo, useRef, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import type { Asset } from '../../api/assets'
import type { Arrival } from '../../hooks/useArrivals'
import { assetPerson, statePill } from '../../lib/asset-model'
import { formatDateTime, formatListTime } from '../../lib/format'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import type { PeopleDirectory } from '../../lib/people'
import { personStyle } from '../../lib/person-hue'
import { PersonChip, Pressable } from '../primitives'
import { AssetPill } from './AssetPill'

const COLS = `grid grid-cols-[minmax(0,1fr)_auto] @2xl:grid-cols-[minmax(0,1fr)_200px_120px_96px]
  gap-x-3 items-center px-2.5`

export interface RealtimeTag {
  /** "Khải vừa cập nhật" */
  text: string
  /** Colour key of the person who did it. Never rendered. */
  hueKey: string
}

interface AssetTableProps {
  assets: Asset[]
  loading: boolean
  people: PeopleDirectory
  openId: string | null
  onOpen: (asset: Asset) => void
  /** Changes with the filters and page; rows enter in sequence when the list is new. */
  scope: string
  fresh: Map<string, Arrival>
  versionOf: (asset: Asset) => string
  tagOf: (asset: Asset, arrival: Arrival) => RealtimeTag | undefined
}

/**
 * The hairline table of assets (mockup §2): tài sản (name over its type) ·
 * người đang giữ · trạng thái · cập nhật, 56px rows, no zebra. A row's name is
 * a real button stretched over the whole row; ↑/↓ move between rows. Below
 * `@2xl` the holder moves under the name.
 */
export function AssetTable({ assets, loading, people, openId, onOpen, scope, fresh, versionOf, tagOf }: AssetTableProps) {
  const m = useMotionPresets()
  const bodyRef = useRef<HTMLDivElement>(null)

  // Rows on screen when the list first shows enter in sequence; later arrivals enter alone.
  const firstBatch = useRef<{ scope: string; ids: Map<string, number> } | null>(null)
  if (!loading && assets.length > 0 && firstBatch.current?.scope !== scope) {
    firstBatch.current = { scope, ids: new Map(assets.map((a, n) => [a.id, n])) }
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
    <div role="table" aria-label="Tài sản" aria-busy={loading || undefined} className="@container grid content-start px-5 pb-2">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Tài sản</span>
        <span role="columnheader" className="hidden @2xl:block">Người đang giữ</span>
        <span role="columnheader" className="text-right @2xl:text-left">Trạng thái</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Cập nhật</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading ? (
          [0, 1, 2, 3, 4].map((i) => (
            <div key={i} role="row" className={`${COLS} h-14 border-b border-line`}>
              <span className="grid gap-1.5">
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${74 - i * 8}%` }} />
                <span className="skeleton h-3 w-1/5 rounded-sm" />
              </span>
              <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
              <span className="skeleton h-5.5 w-20 rounded-full justify-self-end @2xl:justify-self-start" />
              <span className="skeleton h-3.5 w-2/3 rounded-sm ml-auto hidden @2xl:block" />
            </div>
          ))
        ) : (
          <AnimatePresence key={scope}>
            {assets.map((asset) => {
              const index = firstBatch.current?.ids.get(asset.id)
              const first = index !== undefined
              const delay = first ? staggerDelay(index, m.reduced) : 0
              const arrival = fresh.get(versionOf(asset))
              return (
                <motion.div
                  key={asset.id}
                  {...withDelay(first ? m.route : m.row, delay)}
                  exit={m.row.exit}
                  layout={m.layoutProp}
                  transition={m.layout}
                  className="overflow-hidden"
                >
                  <AssetRow
                    asset={asset}
                    people={people}
                    open={openId === asset.id}
                    tag={arrival?.source === 'other' ? tagOf(asset, arrival) : undefined}
                    onOpen={onOpen}
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

const AssetRow = memo(function AssetRow({ asset, people, open, tag, onOpen }: {
  asset: Asset
  people: PeopleDirectory
  open: boolean
  tag?: RealtimeTag
  onOpen: (asset: Asset) => void
}) {
  const holder = asset.assigned_to_user_id ? assetPerson(people, asset.assigned_to_user_id, asset.assigned_to_name) : null
  return (
    <div
      role="row"
      style={tag ? personStyle(tag.hueKey) : undefined}
      className={`${COLS} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'} ${tag ? 'rt-wash' : ''}`}
    >
      <span role="cell" className="grid gap-0.5 min-w-0 py-2">
        <span className="flex items-center gap-2 min-w-0">
          <Pressable
            data-row
            aria-current={open || undefined}
            onClick={() => onOpen(asset)}
            className="min-w-0 line-clamp-2 break-words font-medium text-ink text-left cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {asset.name}
          </Pressable>
          {tag && <span className="rt-tag shrink-0 text-xs font-semibold">{tag.text}</span>}
        </span>
        <small className="text-xs text-ink-muted truncate">
          <span className="@2xl:hidden">{holder ? `${holder.name} · ` : ''}</span>
          {asset.type_name}
        </small>
      </span>
      <span role="cell" className="hidden @2xl:flex min-w-0">
        {holder ? (
          <PersonChip name={holder.name} hueKey={holder.hueKey} avatarUrl={holder.avatarUrl} />
        ) : (
          <span className="text-ink-muted">Chưa giao</span>
        )}
      </span>
      <span role="cell" className="flex justify-end @2xl:justify-start">
        <AssetPill pill={statePill(asset.state)} />
      </span>
      <span role="cell" className="hidden @2xl:block text-right text-ink-muted tnum" title={formatDateTime(asset.updated_at)}>
        {formatListTime(asset.updated_at)}
      </span>
    </div>
  )
})
