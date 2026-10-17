import { CalendarX, CircleAlert } from 'lucide-react'
import { Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'

/** The shape of the grid while it loads: a heading row and a few blocks (DESIGN.md §6, Trạng thái). */
export function GridSkeleton({ columns }: { columns: number }) {
  return (
    <div className="flex flex-col flex-1 min-h-0 px-2" aria-busy="true" aria-label="Đang tải lịch" data-testid="grid-skeleton">
      <div className="grid gap-2 pb-3 pl-14" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
        {Array.from({ length: columns }, (_, i) => (
          <div key={i} className="grid justify-items-center gap-1.5">
            <div className="skeleton h-3 w-6 rounded-sm" />
            <div className="skeleton size-9 rounded-full" />
          </div>
        ))}
      </div>
      <div className="flex-1 grid gap-2 pl-14" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
        {Array.from({ length: columns }, (_, i) => (
          <div key={i} className="relative border-l border-line">
            <div className="skeleton absolute inset-x-1 top-[18%] h-24 rounded-md" />
            <div className="skeleton absolute inset-x-1 top-[52%] h-14 rounded-md" />
          </div>
        ))}
      </div>
    </div>
  )
}

export function CalendarEmpty({ text, onCreate }: { text: string; onCreate?: () => void }) {
  return (
    <div className="flex-1 grid place-items-center" data-testid="calendar-empty">
      <EmptyState
        icon={<CalendarX size={24} strokeWidth={1.75} />}
        text={text}
        action={onCreate && <Button variant="soft" className="rounded-full" onClick={onCreate}>Tạo sự kiện</Button>}
      />
    </div>
  )
}

export function CalendarError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex-1 grid place-items-center" role="alert">
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được lịch. Sự kiện đã lưu không bị mất. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" className="rounded-full" onClick={onRetry}>Thử lại</Button>}
      />
    </div>
  )
}
