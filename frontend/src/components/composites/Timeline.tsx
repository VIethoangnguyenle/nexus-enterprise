import type { ReactNode } from 'react'

interface TimelineItem {
  id: string
  color: string
  title: ReactNode
  timestamp: string
  body?: ReactNode
}

interface TimelineProps {
  items: TimelineItem[]
  className?: string
}

/** Vertical timeline with colored dots and timestamps. */
export function Timeline({ items, className = '' }: TimelineProps) {
  if (items.length === 0) return null

  return (
    <div className={`flex flex-col ${className}`}>
      {items.map(item => (
        <div key={item.id} className="flex gap-3 py-3">
          <div
            className="w-2.5 h-2.5 rounded-full flex-shrink-0 mt-1.5"
            style={{ backgroundColor: item.color }}
          />
          <div className="flex-1 min-w-0">
            <div className="flex items-center justify-between gap-2 mb-0.5">
              <span className="text-sm font-semibold text-text-primary capitalize">
                {item.title}
              </span>
              <span className="text-[0.7rem] text-text-muted flex-shrink-0">
                {item.timestamp}
              </span>
            </div>
            {item.body && (
              <div className="text-xs text-text-secondary">{item.body}</div>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
