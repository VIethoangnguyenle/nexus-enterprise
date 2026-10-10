import { useRef, type KeyboardEvent } from 'react'
import { Pencil } from 'lucide-react'
import type { ApprovalTemplate } from '../../api/approval'
import { entityLabel } from '../../lib/approval-model'
import { formatListTime } from '../../lib/format'
import { IconButton, Pressable } from '../primitives'
import { StatusPill } from './StatusPill'

const COLS = `grid grid-cols-[minmax(0,1fr)_auto_32px] @2xl:grid-cols-[minmax(0,1fr)_144px_88px_112px_32px]
  gap-x-3 items-center px-2.5`

interface TemplatesTableProps {
  templates: ApprovalTemplate[]
  loading: boolean
  openId: string | null
  onOpen: (t: ApprovalTemplate) => void
  onEdit: (t: ApprovalTemplate) => void
}

/** Mẫu phê duyệt as the same hairline table as the requests: name, kind, steps, state. */
export function TemplatesTable({ templates, loading, openId, onOpen, onEdit }: TemplatesTableProps) {
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
    <div role="table" aria-label="Mẫu phê duyệt" aria-busy={loading || undefined} className="@container grid content-start px-5 pb-4">
      <div role="row" className={`${COLS} h-9 text-xs font-semibold text-ink-muted border-b border-line`}>
        <span role="columnheader">Mẫu</span>
        <span role="columnheader" className="hidden @2xl:block">Loại</span>
        <span role="columnheader" className="hidden @2xl:block text-right">Số bước</span>
        <span role="columnheader" className="text-right">Trạng thái</span>
        <span role="columnheader" className="sr-only">Thao tác</span>
      </div>
      <div role="rowgroup" ref={bodyRef} onKeyDown={onKeyDown}>
        {loading
          ? [0, 1, 2].map((i) => (
              <div key={i} role="row" className={`${COLS} h-14 border-b border-line`}>
                <span className="skeleton h-3.5 rounded-sm" style={{ width: `${65 - i * 10}%` }} />
                <span className="skeleton h-3.5 w-3/4 rounded-sm hidden @2xl:block" />
                <span className="skeleton h-3.5 w-1/2 rounded-sm ml-auto hidden @2xl:block" />
                <span className="skeleton h-5.5 w-20 rounded-full ml-auto" />
                <span />
              </div>
            ))
          : templates.map((t) => (
              <div
                key={t.id}
                role="row"
                className={`${COLS} row-lazy relative min-h-14 border-b border-line transition-colors duration-quick
                  ${openId === t.id ? 'bg-accent-wash' : 'hover:bg-hover'}`}
              >
                <span role="cell" className="grid gap-0.5 min-w-0 py-2">
                  <Pressable
                    data-row
                    aria-current={openId === t.id || undefined}
                    onClick={() => onOpen(t)}
                    className="min-w-0 truncate font-medium text-ink text-left cursor-pointer outline-none
                      after:absolute after:inset-0 focus-visible:after:outline-2 focus-visible:after:outline-focus
                      focus-visible:after:-outline-offset-2"
                  >
                    {t.name}
                  </Pressable>
                  <small className="text-xs text-ink-muted tnum truncate">
                    <span className="@2xl:hidden">{entityLabel(t.entity_type)} · </span>
                    Cập nhật {formatListTime(t.updated_at)}
                  </small>
                </span>
                <span role="cell" className="hidden @2xl:block">{entityLabel(t.entity_type)}</span>
                <span role="cell" className="hidden @2xl:block text-right tnum">{t.step_count ?? t.steps?.length ?? 0}</span>
                <span role="cell" className="flex justify-end">
                  <StatusPill pill={t.is_active ? { tone: 'ok', label: 'Đang dùng' } : { tone: 'idle', label: 'Đã tắt' }} />
                </span>
                <span role="cell" className="relative z-dropdown">
                  <IconButton size="md" aria-label={`Sửa mẫu ${t.name}`} title="Sửa mẫu" onClick={() => onEdit(t)}>
                    <Pencil size={16} strokeWidth={1.75} />
                  </IconButton>
                </span>
              </div>
            ))}
      </div>
    </div>
  )
}
