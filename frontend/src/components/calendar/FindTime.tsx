import { useEffect, useMemo, useRef, useState } from 'react'
import { addMinutes, parseISO, startOfDay } from 'date-fns'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { useFreeBusy, useSuggestSlots } from '../../hooks/useCalendarEditor'
import type { GuestPick } from '../../lib/calendar-editor'
import {
  FIND_DAY_PX, FIND_HOUR_PX, boxOnDay, dragSpan, outsideHours, overlapping, resizeSpan, spansOf, statusLine,
} from '../../lib/calendar-findtime'
import { formatClock, formatDayLong, toDateKey } from '../../lib/calendar-format'
import { Avatar, Button, IconButton, Text } from '../primitives'

export interface FindColumn {
  guest: GuestPick
  isMe: boolean
}

interface Props {
  wsId: string
  /** The day shown (the event's first day). */
  day: Date
  /** The event's span, or null while its times are not valid. */
  span: { start: Date; end: Date } | null
  minutes: number
  columns: FindColumn[]
  onSpan: (start: Date, end: Date) => void
  /** Move the whole event by days (the arrows, "Xem ngày mai"). */
  onShiftDay: (days: number) => void
}

const HOURS = Array.from({ length: 24 }, (_, h) => h)

/**
 * Tìm giờ (DESIGN.md §6): a column for each person on the day, the chosen time as an accent band across all of
 * them that can be dragged (or moved with the arrow keys), the busy time of each person, their hours outside
 * work in stripes, and chips for the earliest times everyone is free. Free/busy carries spans only: a block
 * reads "Bận" unless the server was allowed to say more.
 */
export function FindTime({ wsId, day, span, minutes, columns, onSpan, onShiftDay }: Props) {
  const [hidden, setHidden] = useState<Set<string>>(new Set())
  const shown = columns.filter((c) => !hidden.has(c.guest.nodeId))
  const nodes = useMemo(() => shown.map((c) => c.guest.nodeId), [shown])
  const dayStart = startOfDay(day)
  const busyQuery = useFreeBusy(wsId, nodes, dayStart, addMinutes(dayStart, 1440))
  const suggest = useSuggestSlots(wsId, nodes, minutes, toDateKey(day))
  const byNode = useMemo(() => new Map((busyQuery.data ?? []).map((p) => [p.person.nodeId, p])), [busyQuery.data])

  const scroller = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (scroller.current) scroller.current.scrollTop = 7 * FIND_HOUR_PX
  }, [])

  // The band follows the pointer while it is dragged and is committed on release.
  const [preview, setPreview] = useState<{ start: Date; end: Date } | null>(null)
  const drag = useRef<(() => void) | null>(null)
  useEffect(() => () => drag.current?.(), [])
  const live = preview ?? span
  const band = live ? boxOnDay(day, live.start, live.end) : null

  const begin = (mode: 'move' | 'resize') => (e: React.PointerEvent) => {
    if (!span || e.button !== 0) return
    e.preventDefault()
    e.stopPropagation()
    const y0 = e.clientY
    const calc = (ev: PointerEvent) => (mode === 'move' ? dragSpan : resizeSpan)(span.start, span.end, ev.clientY - y0)
    const move = (ev: PointerEvent) => setPreview(calc(ev))
    const up = (ev: PointerEvent) => {
      const r = calc(ev)
      drag.current?.()
      if (r.start.getTime() !== span.start.getTime() || r.end.getTime() !== span.end.getTime()) onSpan(r.start, r.end)
    }
    const stop = () => {
      document.removeEventListener('pointermove', move)
      document.removeEventListener('pointerup', up)
      document.removeEventListener('pointercancel', stop)
      drag.current = null
      setPreview(null)
    }
    document.addEventListener('pointermove', move)
    document.addEventListener('pointerup', up)
    document.addEventListener('pointercancel', stop)
    drag.current = stop
  }

  const onBandKey = (e: React.KeyboardEvent) => {
    if (!span || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return
    e.preventDefault()
    const dy = (e.key === 'ArrowDown' ? 1 : -1) * (FIND_HOUR_PX / 4)
    const r = e.shiftKey ? resizeSpan(span.start, span.end, dy) : dragSpan(span.start, span.end, dy)
    onSpan(r.start, r.end)
  }

  const slots = suggest.data ?? []
  const template = `3.5rem repeat(${Math.max(1, shown.length)}, minmax(9rem, 1fr))`

  return (
    <div className="flex-1 min-h-0 flex flex-col" data-testid="find-time">
      <div className="flex flex-wrap items-center gap-1.5 px-4 pb-2">
        <IconButton size="lg" shape="round" aria-label="Ngày trước" onClick={() => onShiftDay(-1)}><ChevronLeft size={20} strokeWidth={1.75} /></IconButton>
        <IconButton size="lg" shape="round" aria-label="Ngày sau" onClick={() => onShiftDay(1)}><ChevronRight size={20} strokeWidth={1.75} /></IconButton>
        <span className="mr-2 text-lg text-ink">{formatDayLong(day)}</span>
        <span className="flex-1" />
        <span className="text-sm text-ink-muted">Giờ gợi ý</span>
        {suggest.isSuccess && slots.length === 0 && (
          <span className="text-sm text-ink-muted">
            Không có giờ nào mọi người cùng rảnh hôm nay ·{' '}
            <Button type="button" variant="link" size="link" className="text-accent! font-semibold" onClick={() => onShiftDay(1)}>Xem ngày mai</Button>
          </span>
        )}
        {slots.slice(0, 3).map((s) => {
          const a = parseISO(s.start)
          const b = parseISO(s.end)
          return (
            <Button key={s.start} type="button" variant="soft" size="sm" className="rounded-full tnum" onClick={() => onSpan(a, b)}>
              {`${formatClock(a)} – ${formatClock(b)}`}
            </Button>
          )
        })}
      </div>

      {shown.length === 0 ? (
        <Text variant="small" muted className="px-5 py-6">Thêm khách ở tab Chi tiết sự kiện để xem lịch của họ ở đây.</Text>
      ) : (
        <div ref={scroller} className="flex-1 min-h-0 overflow-auto">
          <div className="grid sticky top-0 z-3 bg-base border-b border-line" style={{ gridTemplateColumns: template }}>
            <span />
            {shown.map((c) => {
              const info = byNode.get(c.guest.nodeId)
              const busy = info ? spansOf(info.busy) : []
              const clash = !!live && overlapping(busy, live.start, live.end).length > 0
              return (
                <div key={c.guest.nodeId} className="relative grid justify-items-center gap-0.5 px-2 py-2 text-center">
                  <Avatar name={c.guest.displayName} hue={c.guest.hue} src={c.guest.avatarUrl} size={32} />
                  <span className="max-w-full truncate text-sm font-medium text-ink">{c.guest.displayName}</span>
                  {clash
                    ? <span className="rounded-full bg-warning-wash text-warning text-xs font-semibold px-2">Trùng giờ</span>
                    : <span className="text-xs text-ink-muted">{live ? statusLine(busy, live.start, live.end) : ''}</span>}
                  {!c.isMe && (
                    <IconButton
                      size="sm" className="absolute right-0.5 top-0.5" aria-label={`Ẩn ${c.guest.displayName} khỏi tìm giờ`}
                      onClick={() => setHidden((h) => new Set(h).add(c.guest.nodeId))}
                    >
                      <X size={14} strokeWidth={1.75} />
                    </IconButton>
                  )}
                </div>
              )
            })}
          </div>
          <div className="relative" style={{ height: FIND_DAY_PX }}>
            <div className="grid h-full" style={{ gridTemplateColumns: template }}>
              <div className="relative">
                {HOURS.slice(1).map((h) => (
                  <span key={h} className="absolute right-2 -translate-y-1/2 text-xs text-ink-muted tnum" style={{ top: h * FIND_HOUR_PX }}>
                    {String(h).padStart(2, '0')}:00
                  </span>
                ))}
              </div>
              {shown.map((c) => {
                const info = byNode.get(c.guest.nodeId)
                return (
                  <div key={c.guest.nodeId} className="relative border-l border-line" data-testid={`find-col-${c.isMe ? 'me' : 'guest'}`}>
                    {HOURS.slice(1).map((h) => <span key={h} aria-hidden="true" className="absolute inset-x-0 border-t border-line/60" style={{ top: h * FIND_HOUR_PX }} />)}
                    {info && outsideHours(info).map((b) => (
                      <span key={b.top} aria-hidden="true" className="absolute inset-x-0 busy-stripes opacity-50" style={{ top: b.top, height: b.height }} />
                    ))}
                    {info && spansOf(info.busy).map((b) => {
                      const box = boxOnDay(day, b.start, b.end)
                      if (!box) return null
                      return (
                        <span
                          key={b.start.toISOString() + b.end.toISOString()}
                          className="absolute inset-x-1 rounded-md busy-stripes ring-1 ring-inset ring-line px-1.5 py-0.5 text-xs text-ink-muted overflow-hidden"
                          style={{ top: box.top, height: box.height }}
                        >
                          {b.title ?? 'Bận'}
                        </span>
                      )
                    })}
                  </div>
                )
              })}
            </div>
            {band && live && (
              <div
                role="slider"
                tabIndex={0}
                aria-label={`Khoảng giờ đã chọn, ${formatClock(live.start)} đến ${formatClock(live.end)}`}
                aria-valuetext={`${formatClock(live.start)} – ${formatClock(live.end)}`}
                aria-valuemin={0}
                aria-valuemax={1440}
                aria-valuenow={live.start.getHours() * 60 + live.start.getMinutes()}
                aria-orientation="vertical"
                data-testid="find-band"
                onPointerDown={begin('move')}
                onKeyDown={onBandKey}
                className="absolute left-14 right-0 z-2 rounded-md bg-accent/25 ring-2 ring-accent cursor-grab active:cursor-grabbing touch-none focus-visible:outline-2 focus-visible:outline-focus"
                style={{ top: band.top, height: band.height }}
              >
                <span className="absolute left-2 top-1 text-xs font-semibold text-ink tnum">{`${formatClock(live.start)} – ${formatClock(live.end)}`}</span>
                <span
                  aria-hidden="true"
                  onPointerDown={begin('resize')}
                  className="absolute inset-x-0 bottom-0 h-2 cursor-ns-resize grid place-items-center"
                >
                  <span className="w-8 h-0.5 rounded-full bg-accent" />
                </span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
