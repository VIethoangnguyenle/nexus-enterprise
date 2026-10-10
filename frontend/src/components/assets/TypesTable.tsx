import { memo, useRef, type KeyboardEvent } from 'react'
import { Tag } from 'lucide-react'
import type { AssetType } from '../../api/assets'
import { parseFields } from '../../lib/asset-fields'
import { categoryLabel } from '../../lib/asset-model'
import { Pressable } from '../primitives'

const COLS = `grid grid-cols-[minmax(0,1fr)_auto] @2xl:grid-cols-[minmax(0,1fr)_160px_96px_112px]
  gap-x-3 items-center px-2.5`

interface TypesTableProps {
  types: AssetType[]
  loading: boolean
  openId: string | null
  onOpen: (type: AssetType) => void
}

/** Tên loại · danh mục · tài sản · trường riêng (mockup §5). Below `@2xl` the category sits under the name. */
export function TypesTable({ types, loading, openId, onOpen }: TypesTableProps) {
  const bodyRef = useRef<HTMLDivElement>(null)
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
    <div role="table" aria-label="Loại tài sản" aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Tên loại</span>
        <span role="columnheader" className="hidden @2xl:block">Danh mục</span>
        <span role="columnheader" className="text-right">Tài sản</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Trường riêng</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading
          ? [0, 1, 2].map((i) => (
              <div key={i} role="row" className={`${COLS} h-11 border-b border-line`}>
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${55 - i * 10}%` }} />
                <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
                <span className="skeleton h-3.5 w-8 rounded-sm ml-auto" />
                <span className="skeleton h-3.5 w-8 rounded-sm ml-auto hidden @2xl:block" />
              </div>
            ))
          : types.map((t) => <TypeRow key={t.id} type={t} open={openId === t.id} onOpen={onOpen} />)}
      </div>
    </div>
  )
}

const TypeRow = memo(function TypeRow({ type, open, onOpen }: { type: AssetType; open: boolean; onOpen: (t: AssetType) => void }) {
  const fieldCount = parseFields(type.fields_schema).length
  return (
    <div
      role="row"
      className={`${COLS} row-lazy relative min-h-11 border-b border-line transition-colors duration-quick
        ${open ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span role="cell" className="flex items-center gap-2.5 min-w-0 py-2">
        <Tag size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-muted" />
        <span className="grid min-w-0">
          <Pressable
            data-row
            aria-current={open || undefined}
            onClick={() => onOpen(type)}
            className="min-w-0 truncate font-medium text-ink text-left cursor-pointer outline-none
              after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
              focus-visible:after:-outline-offset-2"
          >
            {type.name}
          </Pressable>
          <small className="@2xl:hidden text-xs text-ink-muted truncate">{categoryLabel(type.category)}</small>
        </span>
      </span>
      <span role="cell" className="hidden @2xl:block text-ink truncate">{categoryLabel(type.category)}</span>
      <span role="cell" className="text-right tnum">{type.asset_count ?? 0}</span>
      <span role="cell" className="hidden @2xl:block text-right tnum">{fieldCount}</span>
    </div>
  )
})
